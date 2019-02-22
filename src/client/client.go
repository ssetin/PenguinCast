// Package iceclient - simple client for icecast server
package iceclient

import (
	"bufio"
	"errors"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	relay2peer "github.com/ssetin/PenguinCast/src/relay2peer"
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
	peerConnector relay2peer.PeerConnector
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
	p.peerConnector.Init(mount, host, "")

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

// Listen - start to listen the stream during secToListen seconds.
// Actually reads bytes according to that duration
func (p *PenguinClient) Listen(secToListen int) error {
	var err error
	var streamConnection net.Conn
	ok := false

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

	// prepare peerConnector to work
	if p.iCan || p.iWant {
		log.Println("prepare peerConnector")
		p.peerConnector.SetIsRelayPoint(p.iCan)
	}

	// if listener find relay point during searchingTimeOut he will use that udp peer connection
	// else he will use direct tcp connection to server
	if p.iWant {
		r2pChan := p.peerConnector.GetConnection()
		streamConnection, ok = <-r2pChan
		if ok {
			log.Println("Found Relay")
			p.conn.Close()
		} else {
			streamConnection = p.conn
		}
	} else {
		streamConnection = p.conn
	}

	defer streamConnection.Close()
	return p.readStream(streamConnection, secToListen)
}

// readStream read stream from appropriated connection
func (p *PenguinClient) readStream(streamConnection net.Conn, secToListen int) error {
	if streamConnection == nil {
		return errors.New("no connection")
	}

	var beginIteration time.Time
	var r2pConnection *relay2peer.PeerConnection
	var r2pChan chan *relay2peer.PeerConnection

	bytesToFinish := secToListen * p.bitRate * 1024 / 8
	readedBytes := 0

	// if streamConnection is relay point, data begins from 7th byte
	startDataIdx := 0
	if _, isPeerConnection := streamConnection.(*relay2peer.PeerConnection); isPeerConnection {
		startDataIdx = 7
	}

	ok := false
	sndBuff := make([]byte, 1024*p.bitRate/8)

	if p.iCan {
		r2pChan = p.peerConnector.GetConnection()
	}

	for readedBytes <= bytesToFinish {
		beginIteration = time.Now()
		n, err := streamConnection.Read(sndBuff)
		if err != nil {
			return err
		}

		// For Relay Point. update latency and try to find listener point
		if p.iCan {
			p.peerConnector.UpdateLatency(int64(time.Since(beginIteration)))

			if r2pConnection == nil {
				select {
				case r2pConnection, ok = <-r2pChan:
					if ok {
						log.Println("Found Listener ")
					} else {
						r2pConnection = nil
					}
				default:
				}
			}

			if r2pConnection != nil {
				p.writeStream(r2pConnection, sndBuff[:n])
			}
		}

		if p.dumpFile != nil && n > 0 {
			p.dumpFile.Write(sndBuff[startDataIdx:n])
		}
		readedBytes += n
	}

	return nil
}

// writeStream relay data to listener point
func (p *PenguinClient) writeStream(conn *relay2peer.PeerConnection, buf []byte) (int, error) {
	return conn.Write(buf)
}
