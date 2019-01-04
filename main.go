package main

import (
	"log"

	"github.com/ssetin/PenguinCast/server"
)

func main() {
	var IcySrv iceserver.IceServer

	err := IcySrv.Init()
	defer IcySrv.Close()
	if err != nil {
		log.Println(err.Error())
		return
	}
	IcySrv.Start()
}
