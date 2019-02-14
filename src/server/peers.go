// Package iceserver - icecast streaming server
package iceserver

import (
	"sort"
	"sync"
)

// ============================== peer ===================================

type peer struct {
	// ip:port
	addr string
	// 0 - ready
	// 1 - busy
	// 2 - disconnected
	status  int
	latency int
	// true if this peer is a relay point
	relay bool
	// index in collection
	idx int
}

type peersCollection []*peer

// Swap elements in collection, recover their indexes
func (c peersCollection) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
	c[i].idx, c[j].idx = i, j
}

func (c peersCollection) Less(i, j int) bool {
	if c[i].latency < c[j].latency && (c[i].status <= c[j].status) {
		return true
	}
	return false
}

func (c peersCollection) Len() int {
	return len(c)
}

// ========================= PeersManager ================================

// PeersManager ...
type PeersManager struct {
	mux sync.Mutex
	// peers, who want to be a p2p relay point
	relayPeers peersCollection
	// peers, who want to be a p2p listener
	listenPeers peersCollection
	// links to peers by their ip
	peers map[string]*peer
}

// Init init collections
func (p *PeersManager) Init() {
	p.relayPeers = make(peersCollection, 0, 10)
	p.listenPeers = make(peersCollection, 0, 10)
	p.peers = make(map[string]*peer)
}

// AddNewRelayPoint add new peer to relays collection
func (p *PeersManager) AddNewRelayPoint(addr string) {
	p.mux.Lock()
	defer p.mux.Unlock()
	if _, ok := p.peers[addr]; ok {
		return
	}
	rp := &peer{addr: addr, relay: true, idx: len(p.relayPeers)}
	p.relayPeers = append(p.relayPeers, rp)
	p.peers[addr] = rp
}

// AddNewListenPoint add new peer to listener collection
func (p *PeersManager) AddNewListenPoint(addr string) {
	p.mux.Lock()
	defer p.mux.Unlock()
	if _, ok := p.peers[addr]; ok {
		return
	}
	lp := &peer{addr: addr, relay: false, idx: len(p.listenPeers)}
	p.listenPeers = append(p.listenPeers, lp)
	p.peers[addr] = lp
}

// deletePeer delete peer from collections
func (p *PeersManager) deletePeer(item *peer) {
	if item.relay {
		p.relayPeers[item.idx] = p.relayPeers[len(p.relayPeers)-1]
		p.relayPeers = p.relayPeers[:len(p.relayPeers)-1]
	} else {
		p.listenPeers[item.idx] = p.listenPeers[len(p.listenPeers)-1]
		p.listenPeers = p.listenPeers[:len(p.listenPeers)-1]
	}
	delete(p.peers, item.addr)
}

// UpdateRelayPoint update peer information
func (p *PeersManager) UpdateRelayPoint(addr string, stat, latc int) {
	p.mux.Lock()
	defer p.mux.Unlock()
	if peer, ok := p.peers[addr]; ok {
		if stat == 2 || latc == -1 {
			p.deletePeer(peer)
		} else {
			peer.latency = latc
			peer.status = stat
		}
	}
}

// GetTop3RelayPoints get top 3 relay point candidate
func (p *PeersManager) GetTop3RelayPoints() []string {
	p.mux.Lock()
	defer p.mux.Unlock()
	result := make([]string, 0, 3)
	sort.Sort(p.relayPeers)

	for i, rp := range p.relayPeers {
		if i > 2 {
			break
		}
		result = append(result, rp.addr)
	}
	return result
}
