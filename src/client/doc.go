// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

/*
Package iceclient - simple client for icecast server

Typical usage

import (
	"log"

	iceclient "github.com/ssetin/PenguinCast/src/client"
)


const (
	mountName = "RockRadio96"
	hostAddr = "127.0.0.1:8008"
)

func main() {
	cl := &iceclient.PenguinClient{}
	cl.Init(hostAddr, mountName, "relay.mp3")

	// listen stream for 300 secs and save it to relay.mp3 file
	err := cl.Listen(300)
	if err != nil {
		log.Println(err)
	}

}

*/
package iceclient
