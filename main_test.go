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

// ================================== Setup ========================================
const (
	listenersCount = 700 // total number of listeners
	incStep        = 50  // number of listeners, to increase with each step
	waitStep       = 2   // seconds between each step
	secToListen    = 240 // seconds to listen by each connection
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
	if err != nil {
		return err
	}
	defer conn.Close()
	if p.dumpFile != nil {
		defer p.dumpFile.Close()
	}

	var headerStr string

	headerStr = "GET /" + mountName + " HTTP/1.0\r\n"
	headerStr += "icy-metadata: 0\r\n"
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
		n, err := conn.Read(sndBuff)
		if err != nil {
			return err
		}

		if p.dumpFile != nil && n > 0 {
			p.dumpFile.Write(sndBuff[:n])
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
	b.ResetTimer()

	for i := 0; i < listenersCount/incStep; i++ {
		wg.Add(incStep)
		for k := 0; k < incStep; k++ {
			go func(wg *sync.WaitGroup, i int) {
				defer wg.Done()
				time.Sleep(time.Millisecond * 129)
				cl := &penguinClient{}
				if i < 30 {
					cl.Init(urlMount, "dump/"+mountName+"."+strconv.Itoa(i)+".mp3")
				} else {
					cl.Init(urlMount, "")
				}
				err := cl.Listen()
				if err != nil {
					log.Println(err)
				}
			}(wg, i)
		}
		time.Sleep(time.Second * waitStep)

	}
	log.Println("Waiting for listeners to finito...")
	wg.Wait()
}

/*
	go test -race -bench . -benchmem main_test.go
	go test -bench . -benchmem -cpuprofile=cpu.out -memprofile=mem.out main_test.go
	mp3check -e -a -S -E -v dump/*.mp3
*/
