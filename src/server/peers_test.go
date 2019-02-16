package iceserver

import (
	"reflect"
	"testing"
)

type SortingCR struct {
	NewRelays    map[string]int
	DeleteRelays []string
	Response     []string
}

func TestManagingPeers(t *testing.T) {
	var manager PeersManager

	manager.Init()

	cases := []SortingCR{SortingCR{
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

	// add existing peer
	manager.AddOrUpdateRelayPoint("192.168.45.19:69919", 0)

	for _, cs := range cases {
		for item := range cs.NewRelays {
			manager.AddOrUpdateRelayPoint(item, 0)
		}
		for item, lat := range cs.NewRelays {
			manager.UpdateRelayPoint(item, 0, lat)
		}

		// sort and get top3
		top3 := manager.GetTop3RelayPoints("")

		// remove after sort
		for _, del := range cs.DeleteRelays {
			manager.UpdateRelayPoint(del, 2, -1)
		}

		// sort again
		top3 = manager.GetTop3RelayPoints("")

		if !reflect.DeepEqual(cs.Response, top3) {
			t.Errorf("Expected: [%v],\ngot [%v]\n", cs.Response, top3)
			continue
		}
	}

}

/*
	go test -v -cover peers_test.go peers.go
	go test -coverprofile=cover.out peers_test.go peers.go
	go tool cover -html=cover.out -o cover.html
*/
