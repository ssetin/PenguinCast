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
	msgHelloRelay    = "helloRelay"
	msgHelloListener = "helloListener"
	keepAliveTimeOut = 500
)

// ReportToServer tells server information about peers
func (p *PenguinClient) ReportToServer() error {
	conn, err := net.Dial("tcp", p.host)
	if err != nil {
		return err
	}
	defer conn.Close()
	writer := bufio.NewWriter(conn)

	writer.WriteString("GET /Pi HTTP/1.0\r\n")
	if p.peerAddr != nil {
		writer.WriteString("Connected: " + p.publicAddr.IP.String() + ":" + strconv.Itoa(p.publicAddr.Port) + ",")
		writer.WriteString(p.peerAddr.String() + "\r\n\r\n")
		writer.Flush()
		return nil
	}

	writer.WriteString("Mount: " + p.mount + "\r\n")
	writer.WriteString("MyAddr: " + p.publicAddr.IP.String() + ":" + strconv.Itoa(p.publicAddr.Port) + "\r\n")
	if p.iCan {
		writer.WriteString("Latency: ")
		writer.WriteString(strconv.Itoa(int(p.latency)))
		writer.WriteString("\r\nFlag: relay\r\n")
	}
	writer.WriteString("\r\n")
	writer.Flush()

	reader := bufio.NewReader(p.conn)
	err = p.processHTTPHeaders(reader)
	if err != nil {
		return err
	}

	return nil
}

func (p *PenguinClient) computeMyIP(msg []byte) error {
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
	p.ReportToServer()
	return nil
}

// Relay2Peer work on p2p transmitting
func (p *PenguinClient) Relay2Peer() error {
	stunAddr, err := net.ResolveUDPAddr("udp", p.stunHost)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	// udp messages channel
	messageChan := p.listenUDP(conn)
	// keep-alive channel
	keepAlive := time.Tick(keepAliveTimeOut * time.Millisecond)

	sentHello := false
	gotHello := false

	for {
		//check, if relay has to be stopped
		if atomic.LoadInt32(&p.Started) == 0 {
			break
		}

		select {
		case message, ok := <-messageChan:
			if !ok {
				return errors.New("error messageChan")
			}

			switch {
			case sentHello && gotHello:
				// process stream
				log.Println(string(message.data))

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
				if p.iCan {
					err = p.sendStr(msgHelloListener, conn, p.peerAddr)
				} else {
					err = p.sendStr(msgHelloRelay, conn, p.peerAddr)
				}
				if err != nil {
					return err
				}
				sentHello = true
			}

		}
	}

	return nil
}

func (p *PenguinClient) sendBindingRequest(conn *net.UDPConn, addr *net.UDPAddr) error {
	m := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	err := p.send(m.Raw, conn, addr)
	if err != nil {
		return fmt.Errorf("binding: %v", err)
	}

	return nil
}

func (p *PenguinClient) send(msg []byte, conn *net.UDPConn, addr *net.UDPAddr) error {
	_, err := conn.WriteToUDP(msg, addr)
	if err != nil {
		return fmt.Errorf("send: %v", err)
	}

	return nil
}

func (p *PenguinClient) sendStr(msg string, conn *net.UDPConn, addr *net.UDPAddr) error {
	return p.send([]byte(msg), conn, addr)
}

func (p *PenguinClient) listenUDP(conn *net.UDPConn) <-chan clientPack {
	messages := make(chan clientPack)
	go func() {
		for {
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
