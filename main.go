package main

import (
	"log"

	"github.com/ssetin/PenguinCast/iceserver"
)

func main() {
	err := icyserver.IcySrv.Init()
	defer icyserver.IcySrv.Close()
	if err != nil {
		log.Println(err.Error())
		return
	}
	icyserver.IcySrv.Start()

}
