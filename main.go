package main

import (
	"log"

	"github.com/ssetin/PenguinCast/src/ice"
)

func main() {
	var IcySrv ice.Server

	err := IcySrv.Init()
	if err != nil {
		log.Println(err.Error())
		return
	}
	defer IcySrv.Close()

	IcySrv.Start()
}
