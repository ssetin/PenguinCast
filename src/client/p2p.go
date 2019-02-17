// Package iceclient - simple client for icecast server
package iceclient

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gortc/stun"
)

const (
	defaultSTUNServer = "stun1.l.google.com:19302"
	msgHelloRelay     = "helloRelay"
	msgHelloListener  = "helloListener"
	keepAliveTimeOut  = 500
	searchingTimeOut  = 2000
)

// PeerConnector responsible for establishing p2p connections
type PeerConnector struct {
	// STUN server address
	stunHost string
	// client public address
	publicAddr stun.XORMappedAddress
	// address of the listener or relay candidate to connect
	peerAddr *net.UDPAddr
	// latency in reading data from the server. the smaller the value,
	// the greater the likelihood that the client will become a relay point
	latency int32
	// isRelayPoint
	isRelayPoint bool

	// connection to radio server
	manConn *net.TCPConn
	// mount name
	mount string

	connChan chan net.Conn
	doneChan chan struct{}
}

type clientPack struct {
	data []byte
	addr *net.UDPAddr
}

// Init ...
func (p *PeerConnector) Init(manSrvConnection *net.TCPConn, mountName string, isRelayPoint bool, stunURL string) {
	if stunURL > "" {
		p.stunHost = stunURL
	} else {
		p.stunHost = defaultSTUNServer
	}
	p.manConn = manSrvConnection
	p.mount = mountName
	p.isRelayPoint = isRelayPoint
	p.peerAddr = nil
	p.doneChan = make(chan struct{})
}

// ReportToServer tells server information about peers
func (p *PeerConnector) reportToServer() error {
	writer := bufio.NewWriter(p.manConn)

	writer.WriteString("GET /Pi HTTP/1.0\r\n")
	if p.peerAddr != nil {
		writer.WriteString("Connected: " + p.publicAddr.IP.String() + ":" + strconv.Itoa(p.publicAddr.Port) + ",")
		writer.WriteString(p.peerAddr.String() + "\r\n\r\n")
		writer.Flush()
		return nil
	}

	writer.WriteString("Mount: " + p.mount + "\r\n")
	writer.WriteString("MyAddr: " + p.publicAddr.IP.String() + ":" + strconv.Itoa(p.publicAddr.Port) + "\r\n")
	if p.isRelayPoint {
		writer.WriteString("Latency: ")
		writer.WriteString(strconv.Itoa(int(p.latency)))
		writer.WriteString("\r\nFlag: relay\r\n")
	}
	writer.WriteString("\r\n")
	writer.Flush()

	reader := bufio.NewReader(p.manConn)
	headers, err := processHTTPHeaders(reader)
	if err != nil {
		log.Println(err.Error())
		return err
	}
	if adr, ok := headers["Address"]; ok {
		p.peerAddr, err = net.ResolveUDPAddr("udp", adr)
	}

	if err != nil {
		return err
	}

	return nil
}

func (p *PeerConnector) computeMyIP(msg []byte) error {
	m := new(stun.Message)
	m.Raw = msg
	err := m.Decode()
	if err != nil {
		return err
	}
	var xorAddr stun.XORMappedAddress
	if getErr := xorAddr.GetFrom(m); getErr != nil {
		return err
	}
	if p.publicAddr.String() != xorAddr.String() {
		p.publicAddr = xorAddr
	}
	p.reportToServer()
	return nil
}

// UpdateLatency ...
func (p *PeerConnector) UpdateLatency(newlat int32) {
	atomic.StoreInt32(&p.latency, newlat)
}

// Relay2Peer returns ready for transmitting udp connection
func (p *PeerConnector) Relay2Peer() chan net.Conn {
	udpConnection := make(chan net.Conn)

	go func() {
		stunAddr, err := net.ResolveUDPAddr("udp", p.stunHost)
		if err != nil {
			log.Println(err)
			close(udpConnection)
			return
		}

		conn, err := net.ListenUDP("udp", nil)
		if err != nil {
			log.Println(err)
			close(udpConnection)
			return
		}

		// udp messages channel
		messageChan := p.listenUDP(conn)
		// keep-alive channel
		keepAlive := time.Tick(keepAliveTimeOut * time.Millisecond)

		sentHello := false
		gotHello := false

		for {
			select {
			case message, ok := <-messageChan:
				if !ok {
					log.Println(errors.New("error messageChan"))
					close(udpConnection)
					return
				}

				switch {
				case sentHello && gotHello:
					// done. pass control to the stream reader
					p.doneChan <- struct{}{}
					udpConnection <- conn
					return

				case string(message.data) == msgHelloListener:
					gotHello = true

				case string(message.data) == msgHelloRelay:
					gotHello = true

				case stun.IsMessage(message.data):
					p.computeMyIP(message.data)

				}

			case <-keepAlive:
				if p.peerAddr == nil {
					p.sendBindingRequest(conn, stunAddr)
				} else {
					// since peer knows another peer address, it should say him hello
					if p.isRelayPoint {
						err = p.sendStr(msgHelloListener, conn, p.peerAddr)
					} else {
						err = p.sendStr(msgHelloRelay, conn, p.peerAddr)
					}
					if err != nil {
						log.Println(errors.New("error messageChan"))
						close(udpConnection)
						return
					}
					sentHello = true
				}

			}
		}
	}()

	return udpConnection
}

func (p *PeerConnector) sendBindingRequest(conn *net.UDPConn, addr *net.UDPAddr) error {
	m := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	err := p.send(m.Raw, conn, addr)
	if err != nil {
		return fmt.Errorf("binding: %v", err)
	}

	return nil
}

func (p *PeerConnector) send(msg []byte, conn *net.UDPConn, addr *net.UDPAddr) error {
	_, err := conn.WriteToUDP(msg, addr)
	if err != nil {
		return fmt.Errorf("send: %v", err)
	}

	return nil
}

func (p *PeerConnector) sendStr(msg string, conn *net.UDPConn, addr *net.UDPAddr) error {
	return p.send([]byte(msg), conn, addr)
}

func (p *PeerConnector) listenUDP(conn *net.UDPConn) <-chan clientPack {
	messages := make(chan clientPack)
	go func() {
		for {
			select {
			case <-p.doneChan:
				close(messages)
				return
			default:
			}

			buf := make([]byte, 1024)
			n, adr, err := conn.ReadFromUDP(buf)
			if err != nil {
				close(messages)
				return
			}
			buf = buf[:n]

			var cl clientPack
			cl.data = buf
			cl.addr = adr

			messages <- cl
		}
	}()
	return messages
}
