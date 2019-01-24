// Package iceclient - simple client for icecast server
// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

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
	cVersion    = "0.1"
)

// PenguinClient ...
type PenguinClient struct {
	urlMount     string
	host         string
	mount        string
	bitRate      int
	dumpFileName string
	dumpFile     *os.File
	conn         net.Conn
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

func (p *PenguinClient) sayHello() error {
	headerStr := "GET /" + p.mount + " HTTP/1.0\r\n"
	headerStr += "icy-metadata: 0\r\n"
	headerStr += "user-agent: " + cClientName + "/" + cVersion + "\r\n"
	headerStr += "accept: */*\r\n"
	headerStr += "\r\n"
	_, err := p.conn.Write([]byte(headerStr))
	return err
}

func (p *PenguinClient) getMountInfo() error {
	reader := bufio.NewReader(p.conn)
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

	err = p.sayHello()
	if err != nil {
		return err
	}

	err = p.getMountInfo()
	if err != nil {
		log.Println(err.Error())
		return err
	}

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
		time.Sleep(time.Millisecond * 100)
	}

	return nil
}
