// Package iceclient - simple client for icecast server
package iceclient

import (
	"bufio"
	"errors"
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
	cClientName = "penguinClient"
	cVersion    = "0.3"
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
	Started  int32
	stunHost string
	// client public address
	publicAddr stun.XORMappedAddress
	// address of the listener or relay candidate to connect
	peerAddr *net.UDPAddr
	// latency in reading data from the server. the smaller the value,
	// the greater the likelihood that the client will become a relay point
	latency int32
	// means that client want to use a relay point
	iWant bool
	// means that client could be a relay point
	iCan bool

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

// SetIWant mark client as a listener point
func (p *PenguinClient) SetIWant(iw bool) {
	p.iWant = iw
}

// SetICan mark client as a relay point
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

func (p *PenguinClient) processHTTPHeaders(reader *bufio.Reader) error {
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
		if len(params) >= 3 {
			// field Address could contain few addresses delimited by [:] in case of several candidates
			// for now i process only one
			if params[0] == "Address" {
				adr := strings.TrimSpace(params[1]) + ":" + strings.TrimSpace(params[2])
				p.peerAddr, err = net.ResolveUDPAddr("udp", adr)
				if err != nil {
					return err
				}
			}
		} else if len(params) == 2 {
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

// Listen - start to listen the stream during secToListen seconds
func (p *PenguinClient) Listen(secToListen int) error {
	var err error
	var beginIteration time.Time
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

	err = p.processHTTPHeaders(reader)
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

	// start p2p conversations
	if p.iCan || p.iWant {
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			p.Relay2Peer()
		}(wg)
	}

	for readedBytes <= bytesToFinish {
		beginIteration = time.Now()
		n, err := p.conn.Read(sndBuff)
		if err != nil {
			atomic.StoreInt32(&p.Started, 0)
			return err
		}
		atomic.StoreInt32(&p.latency, int32(time.Since(beginIteration)))

		if p.dumpFile != nil && n > 0 {
			p.dumpFile.Write(sndBuff[:n])
		}
		readedBytes += n
	}

	atomic.StoreInt32(&p.Started, 0)
	wg.Wait()

	return nil
}
