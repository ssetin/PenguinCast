package icyserver

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	cServerName = "PenguinCast"
	cVersion    = "0.01d"
)

// IcyServer ...
type IcyServer struct {
	serverName string
	version    string

	Props Properties

	logError      *log.Logger
	logErrorFile  *os.File
	logAccess     *log.Logger
	logAccessFile *os.File
}

// Init - Load params from config.json
func (i *IcyServer) Init() error {
	var err error

	i.serverName = cServerName
	i.version = cVersion

	log.Println("Starting " + i.serverName + " " + i.version)

	err = i.initConfig()
	if err != nil {
		return err
	}
	err = i.initLog()
	if err != nil {
		return err
	}
	i.initMounts()
	return nil
}

func (i *IcyServer) initMounts() {
	for idx := range i.Props.Mounts {
		i.Props.Mounts[idx].Init(i)
	}
}

// Close - finish
func (i *IcyServer) Close() {
	log.Print("Stopping " + i.serverName + "...")
	i.logErrorFile.Close()
	i.logAccessFile.Close()
	log.Println("Ok!")
}

func (i *IcyServer) checkIsMount(page string) int {
	for idx := range i.Props.Mounts {
		if filepath.Clean(i.Props.Paths.Web+i.Props.Mounts[idx].Name) == page {
			return idx
		}
	}
	return -1
}

func (i *IcyServer) handler(w http.ResponseWriter, r *http.Request) {
	page, mountidx, err := i.checkPage(w, r)
	if err != nil {
		i.printError(1, err.Error())
		return
	}

	if mountidx >= 0 {
		i.openMount(mountidx, w, r)
		return
	}

	if strings.HasSuffix(page, "info.html") || strings.HasSuffix(page, "info.json") {
		i.renderMounts(w, r, page)
	} else {
		http.ServeFile(w, r, page)
	}
}

/*Start - start listening port ...*/
func (i *IcyServer) Start() {
	http.HandleFunc("/", i.handler)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(i.Props.Socket.Port), nil))
}

/*
IcySrv ...
*/
var IcySrv IcyServer
