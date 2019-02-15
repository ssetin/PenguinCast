// Package iceclient - simple client for icecast server
package iceclient

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gortc/stun"
)

// ================================== PenguinClient ========================================
const (
	cClientName   = "penguinClient"
	cVersion      = "0.3"
	pingMsg       = "ping"
	pongMsg       = "pong"
	timeoutMillis = 500
)

type clientPack struct {
	data []byte
	addr *net.UDPAddr
}

// PenguinClient ...
type PenguinClient struct {
	urlMount     string
	host         string
	mount        string
	bitRate      int
	dumpFileName string

	// p2p relay's staff
	Started    int32
	stunHost   string
	publicAddr stun.XORMappedAddress
	peerAddr   *net.UDPAddr
	iWant      bool
	iCan       bool

	dumpFile *os.File
	conn     net.Conn
}

// Init - initialize client
func (p *PenguinClient) Init(host string, mount string, dump string) error {
	p.urlMount = "http://" + host + "/" + mount
	p.host = host
	p.mount = mount
	p.dumpFileName = dump
	p.bitRate = 0
	p.iWant = false
	p.iCan = false
	p.stunHost = "stun1.l.google.com:19302"
	p.Started = 0
	p.peerAddr = nil

	if dump > "" {
		var err error
		p.dumpFile, err = os.OpenFile(p.dumpFileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			return err
		}
	}

	return nil
}

// SetIWant ...
func (p *PenguinClient) SetIWant(iw bool) {
	p.iWant = iw
}

// SetICan ...
func (p *PenguinClient) SetICan(ic bool) {
	p.iCan = ic
}

func (p *PenguinClient) sayHello(writer *bufio.Writer) error {
	writer.WriteString("GET /")
	writer.WriteString(p.mount)
	writer.WriteString(" HTTP/1.0\r\nicy-metadata: 1\r\nuser-agent: ")
	writer.WriteString(cClientName)
	writer.WriteString("/")
	writer.WriteString(cVersion)
	writer.WriteString("\r\naccept: */*\r\n\r\n")
	writer.Flush()
	return nil
}

func (p *PenguinClient) getPeersAddresses(conn net.Conn) error {
	reader := bufio.NewReader(conn)
	tp := textproto.NewReader(reader)

	for {
		line, err := tp.ReadLine()
		if err != nil {
			return err
		}
		if line == "" {
			break
		}
		params := strings.Split(line, ":")
		if len(params) == 3 {
			if params[0] == "Address" {
				adr := strings.TrimSpace(params[1]) + ":" + strings.TrimSpace(params[2])
				p.peerAddr, err = net.ResolveUDPAddr("udp", adr)
				log.Printf("Got adddr: %s", adr)
				if err != nil {
					return err
				}
			}
		}

	}

	return nil
}

func (p *PenguinClient) getMountInfo(reader *bufio.Reader) error {
	tp := textproto.NewReader(reader)

	for {
		line, err := tp.ReadLine()
		if err != nil {
			return err
		}
		if line == "" {
			break
		}
		params := strings.Split(line, ":")
		if len(params) == 2 {
			if params[0] == "X-Audiocast-Bitrate" {
				p.bitRate, err = strconv.Atoi(strings.TrimSpace(params[1]))
				if err != nil {
					return err
				}
			}
		}

	}

	return nil
}

// SendMyAddr tells server about my external ip and port
func (p *PenguinClient) SendMyAddr(ip, port string) error {
	conn, err := net.Dial("tcp", p.host)
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(conn)

	writer.WriteString("GET /Pi HTTP/1.0\r\n")
	writer.WriteString("Mount: " + p.mount + "\r\n")
	writer.WriteString("MyAddr: " + ip + ":" + port + "\r\n")
	if p.iCan {
		writer.WriteString("Flag: relay\r\n")
	}
	writer.WriteString("\r\n")
	writer.Flush()

	err = p.getPeersAddresses(conn)
	if err != nil {
		return err
	}

	conn.Close()
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
		log.Printf("My public address: %s\n", xorAddr)
		p.publicAddr = xorAddr
	}
	p.SendMyAddr(xorAddr.IP.String(), strconv.Itoa(xorAddr.Port))
	return nil
}

// Relay ...
func (p *PenguinClient) Relay() error {
	srvAddr, err := net.ResolveUDPAddr("udp", p.stunHost)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	messageChan := p.listenUDP(conn)

	keepalive := time.Tick(timeoutMillis * time.Millisecond)

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

			if stun.IsMessage(message.data) {
				p.computeMyIP(message.data)
			} else {
				log.Println(string(message.data))
				err = p.sendStr("HOLA", conn, message.addr)
				if err != nil {
					log.Println(err)
					return err
				}
				time.Sleep(time.Second)
			}

		case <-keepalive:
			if p.peerAddr == nil {
				p.sendBindingRequest(conn, srvAddr)
			} else {
				// Keep NAT binding alive using STUN server or the peer once it's known
				err = p.sendStr("HULALA", conn, p.peerAddr)
				if err != nil {
					log.Println("keepalive:", err)
					return err
				}
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

// Listen - start to listen the stream during secToListen seconds
func (p *PenguinClient) Listen(secToListen int) error {
	var err error
	wg := &sync.WaitGroup{}

	p.conn, err = net.Dial("tcp", p.host)

	if err != nil {
		return err
	}
	defer p.conn.Close()

	if p.dumpFile != nil {
		defer p.dumpFile.Close()
	}

	reader := bufio.NewReader(p.conn)
	writer := bufio.NewWriter(p.conn)

	err = p.sayHello(writer)
	if err != nil {
		return err
	}

	err = p.getMountInfo(reader)
	if err != nil {
		log.Println(err.Error())
		return err
	}
	//p.bitRate = 96
	if p.bitRate == 0 {
		return errors.New("Unknown bitrate")
	}

	bytesToFinish := secToListen * p.bitRate * 1024 / 8
	readedBytes := 0
	sndBuff := make([]byte, 1024*p.bitRate/8)
	atomic.StoreInt32(&p.Started, 1)

	// be a relay point
	if p.iCan || p.iWant {
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			p.Relay()
		}(wg)
	}

	for readedBytes <= bytesToFinish {
		n, err := p.conn.Read(sndBuff)
		if err != nil {
			atomic.StoreInt32(&p.Started, 0)
			return err
		}

		if p.dumpFile != nil && n > 0 {
			p.dumpFile.Write(sndBuff[:n])
		}
		readedBytes += n
	}

	wg.Wait()

	return nil
}
