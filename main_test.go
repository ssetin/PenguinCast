package main

import (
	"io"
	"log"
	"strconv"
	"sync"
	"testing"
	"time"

	iceclient "github.com/ssetin/PenguinCast/src/client"
	iceserver "github.com/ssetin/PenguinCast/src/server"
)

var IcySrv iceserver.IceServer

// ================================== Setup ========================================
const (
	runServer      = true
	listenersCount = 8000 // total number of listeners
	incStep        = 500  // number of listeners, to increase with each step
	waitStep       = 5    // seconds between each step
	secToListen    = 220  // seconds to listen by each connection
	mountName      = "RockRadio96"
	hostAddr       = "192.168.10.2:8008"
)

func startServer() {
	err := IcySrv.Init()
	defer IcySrv.Close()
	if err != nil {
		log.Println(err.Error())
		return
	}
	IcySrv.Start()
}

// ================================== Tests ===========================================

func TestMonitoringListenersCount(b *testing.T) {
	// run server in another process to monitor it separately from clients
	log.Println("Start creating listeners...")

	wg := &sync.WaitGroup{}

	for i := 0; i < listenersCount/incStep; i++ {
		wg.Add(incStep)
		for k := 0; k < incStep; k++ {
			go func(wg *sync.WaitGroup, i int) {
				defer wg.Done()
				time.Sleep(time.Millisecond * 200)
				cl := &iceclient.PenguinClient{}
				if i < 10 {
					cl.Init(hostAddr, mountName, "dump/"+mountName+"."+strconv.Itoa(i)+".mp3")
				} else {
					cl.Init(hostAddr, mountName, "")
				}
				err := cl.Listen(secToListen)
				if err != nil && err != io.EOF {
					log.Println(err)
				}
			}(wg, i*incStep+k)
		}
		time.Sleep(time.Second * waitStep)

	}
	log.Println("Waiting for listeners to finito...")
	wg.Wait()
}

func TestDump(b *testing.T) {
	time.Sleep(time.Second * 5)
	cl := &iceclient.PenguinClient{}
	cl.Init(hostAddr, mountName, "dump/dump2.mp3")
	cl.Listen(35)
}

// ================================== Benchmarks ===========================================
func init() {
	if runServer {
		go startServer()
		time.Sleep(time.Second * 1)
	}
	log.Println("Waiting for SOURCE to connect...")
	time.Sleep(time.Second * 5)
}

func BenchmarkGeneral(b *testing.B) {

	cl := &iceclient.PenguinClient{}
	cl.Init(hostAddr, mountName, "")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := cl.Listen(2)
		if err != nil && err != io.EOF {
			log.Println(err)
		}
	}
}

func BenchmarkParallel(b *testing.B) {
	log.Println("Start creating listeners...")

	wg := &sync.WaitGroup{}

	for i := 0; i < 500/incStep; i++ {
		wg.Add(incStep)
		for k := 0; k < incStep; k++ {
			go func(wg *sync.WaitGroup, i int) {
				defer wg.Done()
				time.Sleep(time.Millisecond * 100)
				cl := &iceclient.PenguinClient{}
				cl.Init(hostAddr, mountName, "")
				err := cl.Listen(120)
				if err != nil && err != io.EOF {
					log.Println(err)
				}
			}(wg, i)
		}
		time.Sleep(time.Second * waitStep)

	}
	log.Println("Waiting for listeners to finito...")
	wg.Wait()
}

func BenchmarkNone(b *testing.B) {
	log.Println("Waiting for listeners...")
	time.Sleep(time.Second * 660)
}

/*
	go test -bench General  -benchmem -benchtime 120s -cpuprofile=cpu.out -memprofile=mem.out main_test.go -run notests
	go test -bench Parallel -race -benchmem -cpuprofile=cpu.out -memprofile=mem.out main_test.go -run notests
	go test -bench None -benchmem -timeout 13m -cpuprofile=cpu.out -memprofile=mem.out main_test.go -run notests

	go tool pprof main.test cpu.out
	go tool pprof -alloc_objects main.test mem.out
	go tool pprof main.test block.out

	go test -v -run MonitoringListenersCount -timeout 10m main_test.go
	go test -v -timeout 60s -run Dump main_test.go

	go-torch main.test cpu.out

	mp3check -e -a -S -T -E -v dump/*.mp3
	ulimit -n 63000
*/
