// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")
package relay2peer

import (
	"hash/crc32"
	"reflect"
	"testing"
)

func TestUDPMessageMarshaling(t *testing.T) {

	StreamData := []byte("kjkjasdfdfkljr78934lui4q mna< o;i<lb <hjrgfklj<rswgkj<gn <<r lk<rgh89rgyp")
	UDPStreamMessageBytes := []byte{82, 50, 80, 121, 42, 254, 145, 107, 106, 107, 106, 97, 115, 100, 102, 100, 102, 107, 108,
		106, 114, 55, 56, 57, 51, 52, 108, 117, 105, 52, 113, 32, 109, 110, 97, 60, 32, 111, 59, 105, 60, 108, 98, 32, 60, 104, 106,
		114, 103, 102, 107, 108, 106, 60, 114, 115, 119, 103, 107, 106, 60, 103, 110, 32, 60, 60, 114, 32, 108, 107, 60, 114, 103, 104, 56,
		57, 114, 103, 121, 112}

	var msg UDPMessage
	msg.FillStreamData(StreamData)

	// check Marshal
	unMarshalledMsg, err := msg.Marshal()
	if err != nil {
		t.Errorf(err.Error())
	}

	if !reflect.DeepEqual(unMarshalledMsg, UDPStreamMessageBytes) {
		t.Errorf("Marshal. Expected: [%v],\ngot [%v]\n", UDPStreamMessageBytes, unMarshalledMsg)
	}

	// check UnMarshal
	err = msg.UnMarshal(UDPStreamMessageBytes)
	if err != nil {
		t.Errorf(err.Error())
	}

	if !reflect.DeepEqual(msg.data, StreamData) {
		t.Errorf("UnMarshal. Expected: [%v],\ngot [%v]\n", StreamData, msg.data)
	}

	expectedCRC := crc32.ChecksumIEEE(StreamData)
	if msg.crc != expectedCRC {
		t.Errorf("Wrong CRC. Expected: [%v], got [%v]\n", expectedCRC, msg.crc)
	}

	if msg.IsStreamMessage() != true {
		t.Errorf("Wrong IsStreamMessage(). Expected: [%v], got [%v]\n", true, msg.IsStreamMessage())
	}

}

/*
	go test -v -cover r2p_test.go r2p.go headers.go
	go test -coverprofile=cover.out r2p_test.go r2p.go
	go tool cover -html=cover.out -o cover.html
*/
