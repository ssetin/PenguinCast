package main

import (
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ssetin/PenguinCast/src/client"
	"github.com/ssetin/PenguinCast/src/server"
)

var IcySrv iceserver.IceServer

// ================================== Setup ========================================
const (
	runServer      = true
	listenersCount = 5000 // total number of listeners
	incStep        = 30   // number of listeners, to increase with each step
	waitStep       = 5    // seconds between each step
	secToListen    = 5400 // seconds to listen by each connection
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
				//if i < 30 {
				//	cl.Init(hostAddr, mountName, "dump/"+mountName+"."+strconv.Itoa(i)+".mp3")
				//} else {
				cl.Init(hostAddr, mountName, "")
				//}
				err := cl.Listen(secToListen)
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

// ================================== Benchmarks ===========================================
func init() {
	if runServer {
		go startServer()
		time.Sleep(time.Second * 1)
	}
	log.Println("Waiting for SOURCE to connect...")
	time.Sleep(time.Second * 5)
}

func BenchmarkSliceReusage01(b *testing.B) {
	reader := strings.NewReader("go test -v -race MonitoringListenersCountgo test -v -race MonitoringListenersCount -timeout 300m main_test.go -timeout 300m main_test.go")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := make([]byte, 2)
		for {
			_, err := reader.Read(p)
			if err != nil {
				break
			}
		}
	}
}

func BenchmarkSliceReusage02(b *testing.B) {
	reader := strings.NewReader("go test -v -race MonitoringListenersCountgo test -v -race MonitoringListenersCount -timeout 300m main_test.go -timeout 300m main_test.go")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for {
			p := make([]byte, 2)
			_, err := reader.Read(p)
			if err != nil {
				break
			}
		}
	}
}

func BenchmarkGeneral(b *testing.B) {

	cl := &iceclient.PenguinClient{}
	cl.Init(hostAddr, mountName, "")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := cl.Listen(40)
		if err != nil {
			log.Println(err)
		}
	}
}

func BenchmarkParallel(b *testing.B) {
	log.Println("Start creating listeners...")

	wg := &sync.WaitGroup{}

	for i := 0; i < 1000/incStep; i++ {
		wg.Add(incStep)
		for k := 0; k < incStep; k++ {
			go func(wg *sync.WaitGroup, i int) {
				defer wg.Done()
				time.Sleep(time.Millisecond * 200)
				cl := &iceclient.PenguinClient{}
				cl.Init(hostAddr, mountName, "")
				err := cl.Listen(300)
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
	go test -bench General  -benchmem -benchtime 120s -cpuprofile=cpu.out -memprofile=mem.out main_test.go -run notests
	go test -bench Parallel -benchmem -cpuprofile=cpu.out -memprofile=mem.out main_test.go -run notests

	go tool pprof main.test cpu.out
	go tool pprof -alloc_objects main.test mem.out
	go tool pprof main.test block.out

	go test -v -run MonitoringListenersCount -timeout 300m main_test.go
	go test -bench Slice -benchmem main_test.go -run notests

	go-torch main.test cpu.out

	mp3check -e -a -S -T -E -v dump/*.mp3
	ulimit -n 63000
*/
