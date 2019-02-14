package iceserver

import (
	"reflect"
	"testing"
)

type SortingCR struct {
	Case     map[string]int
	Response []string
}

func TestManagingPeers(t *testing.T) {
	var manager PeersManager

	manager.Init()

	cases := []SortingCR{SortingCR{
		Case: map[string]int{
			"192.168.45.10:99919": -1, // remove peer
			"192.168.45.13:19919": 110,
			"192.168.45.11:29919": 90,
			"192.168.45.16:39919": 140,
			"192.168.45.10:49919": 7,
			"192.168.45.10:59919": 130,
			"192.168.45.19:69919": 110,
			"192.168.45.66:79919": 12,
			"192.168.45.33:89919": 110,
			"192.168.45.98:99910": 100},
		Response: []string{"192.168.45.10:49919", "192.168.45.66:79919", "192.168.45.11:29919"},
	},
	}

	// existing peer
	manager.AddNewRelayPoint("192.168.45.19:69919")

	for _, cs := range cases {
		for item := range cs.Case {
			manager.AddNewRelayPoint(item)
		}
		for item, lat := range cs.Case {
			manager.UpdateRelayPoint(item, 0, lat)
		}

		top3 := manager.GetTop3RelayPoints()
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
