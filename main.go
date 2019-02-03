package main

import (
	"log"

	iceserver "github.com/ssetin/PenguinCast/src/server"
)

func main() {
	var IcySrv iceserver.IceServer

	err := IcySrv.Init()
	if err != nil {
		log.Println(err.Error())
		return
	}
	defer IcySrv.Close()

	IcySrv.Start()
}
