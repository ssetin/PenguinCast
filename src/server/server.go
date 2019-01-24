// Package iceserver - icecast streaming server
// Copyright 2018 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package iceserver

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ssetin/PenguinCast/src/fastStat"
)

const (
	cServerName = "PenguinCast"
	cVersion    = "0.1dev"
)

var (
	vCommands = [...]string{"metadata"}
)

// IceServer ...
type IceServer struct {
	serverName string
	version    string

	Props Properties

	mux            sync.Mutex
	Started        int32
	StartedTime    time.Time
	ListenersCount int
	SourcesCount   int

	statReader fastStat.ProcStatsReader
	// for monitor
	cpuUsage float64
	memUsage int

	srv           *http.Server
	logError      *log.Logger
	logErrorFile  *os.File
	logAccess     *log.Logger
	logAccessFile *os.File
	statFile      *os.File
}

// Init - Load params from config.json
func (i *IceServer) Init() error {
	var err error

	i.serverName = cServerName
	i.version = cVersion

	i.ListenersCount = 0
	i.SourcesCount = 0
	i.Started = 0

	log.Println("Init " + i.serverName + " " + i.version)

	err = i.initConfig()
	if err != nil {
		return err
	}
	err = i.initLog()
	if err != nil {
		return err
	}
	err = i.initMounts()
	if err != nil {
		return err
	}

	i.srv = &http.Server{
		Addr: ":" + strconv.Itoa(i.Props.Socket.Port),
	}

	if i.Props.Logging.UseStat {
		i.statReader.Init()
		go i.processStats()
	}

	return nil
}

func (i *IceServer) initMounts() error {
	var err error
	for idx := range i.Props.Mounts {
		err = i.Props.Mounts[idx].Init(i)
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *IceServer) incListeners() {
	i.mux.Lock()
	defer i.mux.Unlock()
	i.ListenersCount++
}

func (i *IceServer) decListeners() {
	i.mux.Lock()
	defer i.mux.Unlock()
	if i.ListenersCount > 0 {
		i.ListenersCount--
	}
}

func (i *IceServer) checkListeners() bool {
	i.mux.Lock()
	defer i.mux.Unlock()
	if i.ListenersCount >= i.Props.Limits.Clients {
		return false
	}
	return true
}

func (i *IceServer) incSources() {
	i.mux.Lock()
	defer i.mux.Unlock()
	i.SourcesCount++
}

func (i *IceServer) decSources() {
	i.mux.Lock()
	defer i.mux.Unlock()
	if i.SourcesCount > 0 {
		i.SourcesCount--
	}
}

func (i *IceServer) checkSources() bool {
	i.mux.Lock()
	defer i.mux.Unlock()
	if i.SourcesCount >= i.Props.Limits.Sources {
		return false
	}
	return true
}

// Close - finish
func (i *IceServer) Close() {

	if err := i.srv.Shutdown(context.Background()); err != nil {
		i.printError(1, err.Error())
		log.Printf("Error: %s\n", err.Error())
	} else {
		log.Println("Stopped")
	}

	for idx := range i.Props.Mounts {
		i.Props.Mounts[idx].Close()
	}

	i.statReader.Close()
	i.logErrorFile.Close()
	i.logAccessFile.Close()
	if i.statFile != nil {
		i.statFile.Close()
	}
}

func (i *IceServer) checkIsMount(page string) int {
	for idx := range i.Props.Mounts {
		if i.Props.Mounts[idx].Name == page {
			return idx
		}
	}
	return -1
}

func (i *IceServer) checkIsCommand(page string, r *http.Request) int {
	for idx := range vCommands {
		if vCommands[idx] == page && r.URL.Query().Get("mode") == "updinfo" {
			return idx
		}
	}
	return -1
}

func (i *IceServer) sayHello(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", i.serverName+"/"+i.version)
	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("Allow", "GET, SOURCE")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	flusher.Flush()
}

func (i *IceServer) handler(w http.ResponseWriter, r *http.Request) {
	if i.Props.Logging.Loglevel == 4 {
		i.logHeaders(w, r)
	}

	page, mountidx, cmdidx, err := i.checkPage(w, r)
	if err != nil {
		i.printError(1, err.Error())
		return
	}

	if mountidx >= 0 {
		i.openMount(mountidx, w, r)
		return
	}

	if cmdidx >= 0 {
		i.runCommand(cmdidx, w, r)
		return
	}

	if strings.HasSuffix(page, "info.html") || strings.HasSuffix(page, "info.json") || strings.HasSuffix(page, "monitor.html") {
		i.renderPage(w, r, page)
	} else {
		http.ServeFile(w, r, page)
	}
}

/*
	runCommand
*/
func (i *IceServer) runCommand(idx int, w http.ResponseWriter, r *http.Request) {
	if idx == 0 {
		mountname := path.Base(r.URL.Query().Get("mount"))
		i.printError(4, "runCommand 0 with "+mountname)
		midx := i.checkIsMount(mountname)
		if midx >= 0 {
			i.Props.Mounts[midx].updateMeta(w, r)
		}
	}

}

/*Start - start listening port ...*/
func (i *IceServer) Start() {
	if atomic.LoadInt32(&i.Started) == 1 {
		return
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, os.Kill)

	if i.Props.Logging.UseMonitor {
		http.HandleFunc("/updateMonitor", func(w http.ResponseWriter, r *http.Request) {
			ws, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Fatal(err)
			}
			go i.sendMonitorInfo(ws)
		})
	}

	http.HandleFunc("/", i.handler)

	go func() {
		i.mux.Lock()
		i.StartedTime = time.Now()
		i.mux.Unlock()
		atomic.StoreInt32(&i.Started, 1)
		log.Print("Started on " + i.srv.Addr)

		if err := i.srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}

	}()

	<-stop
	atomic.StoreInt32(&i.Started, 0)
}
