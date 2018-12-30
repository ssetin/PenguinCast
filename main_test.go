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

const listenersCount = 20
const secToListen = 30

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
	conn, err := net.Dial("tcp", "127.0.0.1:8008")
	defer conn.Close()

	var headerBuf string
	sndBuff := make([]byte, 1024*96/8)

	headerBuf = "GET /RockRadio96 HTTP/1.0\r\n"
	headerBuf += "icy-metadata: 1\r\n"
	headerBuf += "user-agent: penguinClient/0.1\r\n"
	headerBuf += "accept: */*\r\n"
	headerBuf += "\r\n"
	_, err = conn.Write([]byte(headerBuf))
	if err != nil {
		return err
	}

	bytesToFinish := secToListen * 96 * 1024 / 8
	readedBytes := 0

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

	if p.dumpFile != nil {
		p.dumpFile.Close()
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
			cl.Init("http://127.0.0.1:8008/RockRadio96", "dump/dump"+strconv.Itoa(i)+".mp3")
			err := cl.Listen()
			if err != nil {
				log.Fatalln(err)
			}
		}(wg, i)
		time.Sleep(time.Millisecond * 500)

	}
	log.Println("Waiting for listeners to finito...")
	wg.Wait()
	IcySrv.Close()
}

/*
	go test -bench . -benchmem main_test.go
*/
