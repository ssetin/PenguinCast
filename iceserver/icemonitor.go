package iceserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

//MonitorInfo ...
type MonitorInfo struct {
	Mounts []MountInfo
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (i *IceServer) sendMonitorInfo(client *websocket.Conn) {
	ticker := time.NewTicker(4 * time.Second)
	for {
		w, err := client.NextWriter(websocket.TextMessage)
		if err != nil {
			ticker.Stop()
			break
		}

		monitorInfo := &MonitorInfo{}
		monitorInfo.Mounts = make([]MountInfo, 0, len(i.Props.Mounts))

		for idx := range i.Props.Mounts {
			inf := i.Props.Mounts[idx].getMountsInfo()
			monitorInfo.Mounts = append(monitorInfo.Mounts, inf)
		}

		msg, _ := json.Marshal(monitorInfo)
		w.Write(msg)
		w.Close()

		<-ticker.C
	}
}
