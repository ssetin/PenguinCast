// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package ice

import (
	"encoding/json"
	"math"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

//MonitorInfo ...
type monitorInfo struct {
	Mounts   []mountInfo
	CPUUsage float64
	MemUsage int
}

var upGrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (i *Server) processStats() {
	ticker := time.NewTicker(time.Duration(i.Options.Logging.StatInterval) * time.Second)
	for {
		CPU, Memory, err := i.statReader.GetCPUAndMem()
		if err != nil {
			i.logger.Error(err.Error())
			ticker.Stop()
			break
		}
		i.logger.Stat("%d\t%f\t%d\n", atomic.LoadInt32(&i.ListenersCount), CPU, Memory/1024)
		i.mux.Lock()
		i.cpuUsage = math.Floor(CPU*100) / 100
		i.memUsage = Memory / 1024
		i.mux.Unlock()
		<-ticker.C
	}
}

func (i *Server) sendMonitorInfo(interval int, client *websocket.Conn) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	for {
		w, err := client.NextWriter(websocket.TextMessage)
		if err != nil {
			ticker.Stop()
			break
		}

		monitorInfo := &monitorInfo{}
		monitorInfo.Mounts = make([]mountInfo, 0, len(i.Options.Mounts))

		for idx := range i.Options.Mounts {
			inf := i.Options.Mounts[idx].getMountsInfo()
			monitorInfo.Mounts = append(monitorInfo.Mounts, inf)
		}
		i.mux.Lock()
		monitorInfo.CPUUsage = i.cpuUsage
		monitorInfo.MemUsage = i.memUsage
		i.mux.Unlock()

		msg, _ := json.Marshal(monitorInfo)
		w.Write(msg)
		w.Close()

		<-ticker.C
	}
}
