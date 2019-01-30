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
	"time"
)

// ================================== PenguinClient ========================================
const (
	cClientName = "penguinClient"
	cVersion    = "0.2"
)

// PenguinClient ...
type PenguinClient struct {
	urlMount     string
	host         string
	mount        string
	bitRate      int
	dumpFileName string

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

	if dump > "" {
		var err error
		p.dumpFile, err = os.OpenFile(p.dumpFileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			return err
		}
	}

	return nil
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

// Listen - start to listen the stream during secToListen seconds
func (p *PenguinClient) Listen(secToListen int) error {
	var err error
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

	for readedBytes <= bytesToFinish {
		n, err := p.conn.Read(sndBuff)
		if err != nil {
			return err
		}

		if p.dumpFile != nil && n > 0 {
			p.dumpFile.Write(sndBuff[:n])
		}
		readedBytes += n
		time.Sleep(time.Millisecond * 500)
	}

	return nil
}
