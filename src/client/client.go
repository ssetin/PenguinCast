// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package iceclient

import (
	"bufio"
	"errors"
	"log"
	"net"
	"os"
	"strconv"
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

	dumpFile *os.File
	conn     net.Conn
}

// Init - initialize client
func (p *PenguinClient) Init(host string, mount string, dump string) error {
	p.urlMount = "http://" + host + "/" + mount
	p.host = host
	p.mount = mount
	p.dumpFileName = dump

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

// Listen - start to listen the stream during secToListen seconds.
// Actually reads bytes according to that duration
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
		return errors.New("unknown bitrate")
	}

	streamConnection = p.conn

	defer streamConnection.Close()
	return p.readStream(streamConnection, secToListen)
}

// readStream read stream from appropriated connection
func (p *PenguinClient) readStream(streamConnection net.Conn, secToListen int) error {
	if streamConnection == nil {
		return errors.New("no connection")
	}

	bytesToFinish := secToListen * p.bitRate * 1024 / 8
	readedBytes := 0
	packSize := 0

	sndBuff := make([]byte, 1024*p.bitRate/8)

	for readedBytes <= bytesToFinish {
		n, err := streamConnection.Read(sndBuff)
		if err != nil {
			return err
		}

		if p.dumpFile != nil && n > 0 {
			p.dumpFile.Write(sndBuff[:n])
		}
		readedBytes += n
		packSize += n
	}

	return nil
}
