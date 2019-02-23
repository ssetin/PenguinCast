// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package iceserver

import (
	"reflect"
	"testing"
	"time"
)

type SortingCR struct {
	NewRelays    map[string]int
	DeleteRelays []string
	Response     []string
}

var (
	logWrited bool
)

func dummyLog(host string, startTime time.Time, request string, bytessended int, refer, userAgent string, seconds int) {
	logWrited = true
}

func TestManagingPeers(t *testing.T) {
	var manager PeersManager
	var top3 []string
	adrListener := "192.168.45.99:77777"
	candidates := []string{"fake"}

	manager.Init(dummyLog, "test", 2, 1)
	defer manager.Close()

	// add listener
	if manager.AddListenPoint(adrListener) != true {
		t.Error("Add new peer. Expected: true, got: false\n")
	}

	// add existing listener
	if manager.AddListenPoint(adrListener) != false {
		t.Error("Add new peer. Expected: false, got: true\n")
	}

	// delete listener
	manager.deletePeer(manager.GetPeer(adrListener))

	// add listener again
	if manager.AddListenPoint(adrListener) != true {
		t.Error("Add new peer. Expected: true, got: false\n")
	}
	manager.AddListenPoint("fake")

	cases := []SortingCR{{
		NewRelays: map[string]int{
			"192.168.45.13:19919": 110,
			"192.168.45.16:39919": 140,
			"192.168.45.10:49919": 7,
			"192.168.45.10:99919": 99,
			"192.168.45.10:59919": 130,
			"192.168.45.19:69919": 110,
			"192.168.45.11:29919": 90,
			"192.168.45.33:89919": 110,
			"192.168.45.66:79919": 12,
			"192.168.45.98:99910": 100},
		DeleteRelays: []string{"192.168.45.11:29919", "192.168.45.10:99919"},
		Response:     []string{"192.168.45.10:49919", "192.168.45.66:79919", "192.168.45.98:99910"},
	},
	}

	// adding, sorting and getting
	for _, cs := range cases {
		for item := range cs.NewRelays {
			manager.AddOrUpdateRelayPoint(item, 0)
		}
		for item, lat := range cs.NewRelays {
			manager.UpdateRelayPoint(item, 0, lat)
		}

		// sort and "get" top3
		manager.GetTop3RelayPoints("fake")

		// remove after sort
		for _, del := range cs.DeleteRelays {
			manager.UpdateRelayPoint(del, 2, -1)
		}

		// sort again
		top3 = manager.GetTop3RelayPoints(adrListener)

		if !reflect.DeepEqual(cs.Response, top3) {
			t.Errorf("Expected: [%v],\ngot [%v]\n", cs.Response, top3)
			continue
		}
	}

	// add existing peer
	if manager.AddOrUpdateRelayPoint("192.168.45.19:69919", 0) != false {
		t.Error("Add existing peer. Expected: false, got: true\n")
	}

	// get missing peer
	if manager.GetPeer("199.168.45.99:77777") != nil {
		t.Error("Get missing peer. Expected: nil, got: not nil\n")
	}

	// connect peers
	manager.PeersConnected(adrListener, "192.168.45.10:49919")
	peer1 := manager.GetPeer("192.168.45.10:49919")
	peer2 := manager.GetPeer(adrListener)

	if peer1.status != 1 && peer2.status != 1 {
		t.Errorf("Connect peers. Expected: 1 and 1, got: %d and %d\n", peer1.status, peer2.status)
	}

	// check update time
	time.Sleep(time.Millisecond * 100)
	peer2.Update()
	if !peer2.lastUpdateTime.After(peer2.connectedTime) {
		t.Errorf("Check update time. Expected: lastUpdateTime [%v] > connectedTime [%v]\n", peer2.lastUpdateTime, peer2.connectedTime)
	}

	// check candidates list
	clist := peer1.GetCandidates()
	if !reflect.DeepEqual(clist, candidates) {
		t.Errorf("Check candidates. Expected: [%v], got: [%v]\n", candidates, clist)
	}

	// add nil candidate
	preLen := len(peer1.candidates)
	peer1.AddCandidate(nil)
	if preLen < len(peer1.candidates) {
		t.Errorf("Add nil candidate. Expected: %d, got: %d\n", preLen, len(peer1.candidates))
	}

	// clear candidates
	peer1.ClearCandidates()
	if len(peer1.candidates) != 0 {
		t.Errorf("Clear candidates. Expected: 0, got: %d\n", len(peer1.candidates))
	}

	// check inspector
	time.Sleep(time.Second * 4)
	peer2 = manager.GetPeer(adrListener)
	if peer2 != nil {
		t.Error("Check inspector. Expected: nil, got: not nil\n")
	}

	// check log callback
	if logWrited != true {
		t.Error("Check log callback. Expected: true, got:false\n")
	}

}

/*
	go test -v -cover peers_test.go peers.go
	go test -v -coverprofile=cover.out peers_test.go peers.go
	go tool cover -html=cover.out -o cover.html
*/
