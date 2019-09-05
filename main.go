package main

import (
	"log"

	"github.com/ssetin/PenguinCast/src/ice"
)

func main() {
	server, err := ice.NewServer()

	if err != nil {
		log.Println(err.Error())
		return
	}
	defer server.Close()

	server.Start()
}
