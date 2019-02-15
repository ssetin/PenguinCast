// Package iceserver - icecast streaming server
package iceserver

import (
	"sort"
	"sync"
)

// ============================== peer ===================================

// Peer ...
type Peer struct {
	mux sync.Mutex
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
	// candidates to connect
	// when this peer is relay point this field contains possible listeners
	candidates []*Peer
}

// AddCandidate add listener who get this peer as possible relay point
func (p *Peer) AddCandidate(listener *Peer) {
	p.mux.Lock()
	defer p.mux.Unlock()
	if p.candidates == nil {
		p.candidates = make([]*Peer, 0, 3)
	}
	p.candidates = append(p.candidates, listener)
}

// ClearCandidates delete all candidates
func (p *Peer) ClearCandidates() {
	p.mux.Lock()
	defer p.mux.Unlock()
	p.candidates = nil
}

// SetStatus set status
func (p *Peer) SetStatus(newstat int) {
	p.mux.Lock()
	defer p.mux.Unlock()
	p.status = newstat
}

// SetLatency set latency
func (p *Peer) SetLatency(newlat int) {
	p.mux.Lock()
	defer p.mux.Unlock()
	p.latency = newlat
}

// SetStatusLatency set latency and status
func (p *Peer) SetStatusLatency(newstat, newlat int) {
	p.mux.Lock()
	defer p.mux.Unlock()
	p.latency = newlat
	p.status = newstat
}

// GetCandidates get candidates addresses to connect
func (p *Peer) GetCandidates() []string {
	result := make([]string, 0, 3)
	p.mux.Lock()
	defer p.mux.Unlock()

	if p.candidates != nil {
		for i, rp := range p.candidates {
			if i > 2 || rp.status > 0 {
				break
			}
			result = append(result, rp.addr)
		}
	}
	return result
}

// ========================== peersCollection ==============================

type peersCollection []*Peer

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
	peers map[string]*Peer
}

// Init init collections
func (p *PeersManager) Init() {
	p.relayPeers = make(peersCollection, 0, 10)
	p.listenPeers = make(peersCollection, 0, 10)
	p.peers = make(map[string]*Peer)
}

// AddNewRelayPoint add new peer to relays collection
func (p *PeersManager) AddNewRelayPoint(addr string) bool {
	p.mux.Lock()
	defer p.mux.Unlock()
	if _, ok := p.peers[addr]; ok {
		return false
	}
	rp := &Peer{addr: addr, relay: true, idx: len(p.relayPeers)}
	p.relayPeers = append(p.relayPeers, rp)
	p.peers[addr] = rp
	return true
}

// AddNewListenPoint add new peer to listener collection
func (p *PeersManager) AddNewListenPoint(addr string) bool {
	p.mux.Lock()
	defer p.mux.Unlock()
	if _, ok := p.peers[addr]; ok {
		return false
	}
	lp := &Peer{addr: addr, relay: false, idx: len(p.listenPeers)}
	p.listenPeers = append(p.listenPeers, lp)
	p.peers[addr] = lp
	return true
}

// GetPeer get peer by address
func (p *PeersManager) GetPeer(addr string) *Peer {
	p.mux.Lock()
	defer p.mux.Unlock()
	if peer, ok := p.peers[addr]; ok {
		return peer
	}
	return nil
}

// deletePeer delete peer from collections
func (p *PeersManager) deletePeer(item *Peer) {
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
			peer.SetStatusLatency(stat, latc)
		}
	}
}

// PeersConnected mark peers as busy
func (p *PeersManager) PeersConnected(addr, addr2 string) {
	p.mux.Lock()
	defer p.mux.Unlock()
	if peer, ok := p.peers[addr]; ok {
		peer.SetStatus(1)
	}
	if peer, ok := p.peers[addr2]; ok {
		peer.SetStatus(1)
	}
}

// GetTop3RelayPoints get top 3 relay point candidate
func (p *PeersManager) GetTop3RelayPoints(listenerAddr string) []string {
	p.mux.Lock()
	defer p.mux.Unlock()
	result := make([]string, 0, 3)
	sort.Sort(p.relayPeers)

	listenerPeer := p.peers[listenerAddr]

	for i, rp := range p.relayPeers {
		if i > 2 || rp.status > 0 {
			break
		}
		rp.AddCandidate(listenerPeer)
		result = append(result, rp.addr)
	}
	return result
}
