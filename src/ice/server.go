// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package ice

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ssetin/PenguinCast/src/log"

	"github.com/ssetin/PenguinCast/src/config"

	"github.com/ssetin/PenguinCast/src/pool"
	"github.com/ssetin/PenguinCast/src/stat"
)

const (
	cServerName = "PenguinCast"
	cVersion    = "0.3.0"
)

var (
	vCommands = [...]string{"metadata"}
)

// Server ...
type Server struct {
	serverName string
	version    string

	Options config.Options
	Mounts  []Mount

	mux            sync.Mutex
	Started        int32
	StartedTime    time.Time
	ListenersCount int32
	SourcesCount   int32

	statReader stat.ProcStatsReader
	// for monitor
	cpuUsage float64
	memUsage int

	srv         *http.Server
	poolManager pool.Manager
	logger      *log.IceLogger
}

// Init - Load params from config.json
func (i *Server) Init() error {
	var err error

	i.serverName = cServerName
	i.version = cVersion

	i.ListenersCount = 0
	i.SourcesCount = 0
	i.Started = 0

	err = i.Options.Load()
	if err != nil {
		return err
	}

	i.logger, err = log.NewLogger(log.LogsLevel(i.Options.Logging.Loglevel), i.Options.Paths.Log)
	if err != nil {
		return err
	}
	err = i.initMounts()
	if err != nil {
		return err
	}

	i.logger.Log("Init " + i.serverName + " " + i.version)

	i.srv = &http.Server{
		Addr: ":" + strconv.Itoa(i.Options.Socket.Port),
	}

	if i.Options.Logging.UseStat {
		i.statReader.Init()
		go i.processStats()
	}

	return nil
}

func (i *Server) initMounts() error {
	var err error
	i.Mounts = make([]Mount, len(i.Options.Mounts))
	for idx := range i.Mounts {
		err = i.Mounts[idx].Init(i, i.Options.Mounts[idx])
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *Server) incListeners() {
	atomic.AddInt32(&i.ListenersCount, 1)
}

func (i *Server) decListeners() {
	if atomic.LoadInt32(&i.ListenersCount) > 0 {
		atomic.AddInt32(&i.ListenersCount, -1)
	}
}

func (i *Server) checkListeners() bool {
	clientsLimit := atomic.LoadInt32(&i.Options.Limits.Clients)
	if atomic.LoadInt32(&i.ListenersCount) > clientsLimit {
		return false
	}
	return true
}

func (i *Server) incSources() {
	atomic.AddInt32(&i.SourcesCount, 1)
}

func (i *Server) decSources() {
	if atomic.LoadInt32(&i.SourcesCount) > 0 {
		atomic.AddInt32(&i.SourcesCount, -1)
	}
}

func (i *Server) checkSources() bool {
	sourcesLimit := atomic.LoadInt32(&i.Options.Limits.Sources)
	if atomic.LoadInt32(&i.SourcesCount) > sourcesLimit {
		return false
	}
	return true
}

// Close - finish
func (i *Server) Close() {
	if err := i.srv.Shutdown(context.Background()); err != nil {
		i.logger.Error(1, err.Error())
		i.logger.Log("Error: %s\n", err.Error())
	} else {
		i.logger.Log("Stopped")
	}

	for idx := range i.Mounts {
		i.Mounts[idx].Close()
	}

	i.statReader.Close()
	i.logger.Close()
}

func (i *Server) checkIsMount(page string) int {
	for idx := range i.Mounts {
		if i.Mounts[idx].Options.Name == page {
			return idx
		}
	}
	return -1
}

func (i *Server) getMount(mountName string) *Mount {
	for idx := range i.Mounts {
		if i.Mounts[idx].Options.Name == mountName {
			return &i.Mounts[idx]
		}
	}
	return nil
}

func (i *Server) checkIsCommand(page string, r *http.Request) int {
	for idx := range vCommands {
		if vCommands[idx] == page && r.URL.Query().Get("mode") == "updinfo" {
			return idx
		}
	}
	return -1
}

// handler general http handler
func (i *Server) handler(w http.ResponseWriter, r *http.Request) {
	if i.Options.Logging.Loglevel == 4 {
		i.logHeaders(w, r)
	}

	page, mountIdx, cmdIdx, err := i.checkPage(w, r)
	if err != nil {
		i.logger.Error(1, err.Error())
		return
	}

	if mountIdx >= 0 {
		i.openMount(mountIdx, w, r)
		return
	}

	if cmdIdx >= 0 {
		i.runCommand(cmdIdx, w, r)
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
func (i *Server) runCommand(idx int, w http.ResponseWriter, r *http.Request) {
	if idx == 0 {
		mountName := path.Base(r.URL.Query().Get("mount"))
		i.logger.Error(4, "runCommand 0 with "+mountName)
		mIdx := i.checkIsMount(mountName)
		if mIdx >= 0 {
			i.Mounts[mIdx].updateMeta(w, r)
		}
	}

}

/*Start - start listening port ...*/
func (i *Server) Start() {
	if atomic.LoadInt32(&i.Started) == 1 {
		return
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, os.Kill)

	if i.Options.Logging.UseMonitor {
		http.HandleFunc("/updateMonitor", func(w http.ResponseWriter, r *http.Request) {
			ws, err := upGrader.Upgrade(w, r, nil)
			if err != nil {
				panic(err)
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
		i.logger.Log("Started on " + i.srv.Addr)

		if err := i.srv.ListenAndServe(); err != http.ErrServerClosed {
			panic(err)
		}

	}()

	<-stop
	atomic.StoreInt32(&i.Started, 0)
}
