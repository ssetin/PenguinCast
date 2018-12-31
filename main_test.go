package main

import (
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ssetin/PenguinCast/iceserver"
)

const (
	listenersCount = 200
	secToListen    = 60
	mountName      = "RockRadio96"
	bitRate        = 96
	hostAddr       = "127.0.0.1:8008"
	urlMount       = "http://" + hostAddr + "/" + mountName
)

var IcySrv iceserver.IceServer

// ================================== penguinClient ========================================

type penguinClient struct {
	urlMount     string
	dumpFileName string
	dumpFile     *os.File
}

func (p *penguinClient) Init(url string, dump string) error {
	p.urlMount = url
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

func (p *penguinClient) Listen() error {
	conn, err := net.Dial("tcp", hostAddr)
	defer conn.Close()
	if p.dumpFile != nil {
		defer p.dumpFile.Close()
	}

	var headerStr string

	headerStr = "GET /" + mountName + " HTTP/1.0\r\n"
	headerStr += "icy-metadata: 1\r\n"
	headerStr += "user-agent: penguinClient/0.1\r\n"
	headerStr += "accept: */*\r\n"
	headerStr += "\r\n"
	_, err = conn.Write([]byte(headerStr))
	if err != nil {
		return err
	}

	bytesToFinish := secToListen * bitRate * 1024 / 8
	readedBytes := 0
	sndBuff := make([]byte, 1024*bitRate/8)

	for readedBytes <= bytesToFinish {
		time.Sleep(time.Second)
		n, err := conn.Read(sndBuff)
		if err != nil {
			return err
		}

		if p.dumpFile != nil && n > 0 {
			p.dumpFile.Write(sndBuff)
		}
		readedBytes += n
	}

	return nil
}

func startServer() {
	err := IcySrv.Init()
	defer IcySrv.Close()
	if err != nil {
		log.Println(err.Error())
		return
	}
	IcySrv.Start()
}

// ================================== Benchmarks ===========================================

func BenchmarkListenersCount(b *testing.B) {
	go startServer()
	time.Sleep(time.Second * 2)

	log.Println("Waiting for SOURCE to connect...")
	time.Sleep(time.Second * 10)
	log.Println("Start creating listeners...")

	wg := &sync.WaitGroup{}

	for i := 0; i < listenersCount; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, i int) {
			defer wg.Done()

			cl := &penguinClient{}
			cl.Init(urlMount, "dump/"+mountName+"."+strconv.Itoa(i)+".mp3")
			err := cl.Listen()
			if err != nil {
				log.Println(err)
			}
		}(wg, i)
		time.Sleep(time.Millisecond * 100)

	}
	log.Println("Waiting for listeners to finito...")
	wg.Wait()
	IcySrv.Close()
}

/*
	go test -bench . -benchmem main_test.go
*/
