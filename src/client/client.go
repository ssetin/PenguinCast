// Package iceclient - simple client for icecast server
package iceclient

import (
	"bufio"
	"errors"
	"log"
	"net"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

// ================================== PenguinClient ========================================
const (
	cClientName = "penguinClient"
	cVersion    = "0.3"
)

// PenguinClient ...
type PenguinClient struct {
	urlMount     string
	host         string
	mount        string
	dumpFileName string
	bitRate      int

	// responsible for establishing p2p connections
	peerConnector PeerConnector
	//
	Started int32
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
	p.iWant = false
	p.iCan = false
	p.Started = 0

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

// Listen - start to listen the stream during secToListen seconds
func (p *PenguinClient) Listen(secToListen int) error {
	var err error
	var streamConnection net.Conn

	p.conn, err = net.Dial("tcp", p.host)
	if err != nil {
		return err
	}

	if p.dumpFile != nil {
		defer p.dumpFile.Close()
	}

	reader := bufio.NewReader(p.conn)
	writer := bufio.NewWriter(p.conn)

	err = p.sayHello(writer)
	if err != nil {
		p.conn.Close()
		return err
	}

	headers, err := processHTTPHeaders(reader)
	if err != nil {
		p.conn.Close()
		log.Println(err.Error())
		return err
	}
	p.bitRate, err = strconv.Atoi(headers["X-Audiocast-Bitrate"])
	//p.bitRate = 96
	if err != nil || p.bitRate == 0 {
		p.conn.Close()
		return errors.New("Unknown bitrate")
	}

	// if listener find relay point during searchingTimeOut he will use that udp peer connection
	// else he will use direct tcp connection to server
	if p.iWant {
		streamConnection = <-p.peerConnector.Relay2Peer()
		if streamConnection != nil {
			p.conn.Close()
		} else {
			streamConnection = p.conn
		}
	} else {
		streamConnection = p.conn
	}

	err = p.readStream(streamConnection, secToListen)
	if err != nil {
		return err
	}

	streamConnection.Close()

	return nil
}

// readStream ...
func (p *PenguinClient) readStream(conn net.Conn, secToListen int) error {
	var beginIteration time.Time
	var r2pConnection net.Conn
	bytesToFinish := secToListen * p.bitRate * 1024 / 8
	readedBytes := 0
	sndBuff := make([]byte, 1024*p.bitRate/8)

	for readedBytes <= bytesToFinish {
		beginIteration = time.Now()
		n, err := conn.Read(sndBuff)
		if err != nil {
			atomic.StoreInt32(&p.Started, 0)
			return err
		}

		// update latency and try to find listener point
		if p.iCan {
			p.peerConnector.UpdateLatency(int32(time.Since(beginIteration)))

			if r2pConnection == nil {
				select {
				case r2pConnection = <-p.peerConnector.Relay2Peer():
				default:
				}
			}

			if r2pConnection != nil {
				p.writeStream(r2pConnection, sndBuff, n)
			}
		}

		if p.dumpFile != nil && n > 0 {
			p.dumpFile.Write(sndBuff[:n])
		}
		readedBytes += n
	}

	return nil
}

// writeStream ...
func (p *PenguinClient) writeStream(conn net.Conn, buf []byte, n int) {
	conn.Write(buf[:n])
}
