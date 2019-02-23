// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package iceserver

import (
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ============================== peer ===================================

const (
	peerLifeTimeUpdateTimeOut = 15 // sec
)

// Peer ...
type Peer struct {
	mux sync.Mutex
	// ip:port
	addr string
	// 0 - ready
	// 1 - connected
	// 2 - disconnected
	status int
	// latency in reading data from the server for that peer
	latency int
	// true if this peer is a relay point
	relay bool
	// index in collection
	idx int
	// candidates to connect
	// when this peer is relay point this field contains possible listeners
	candidates []*Peer

	connectedTime  time.Time
	lastUpdateTime time.Time
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
	if newstat == 1 {
		p.connectedTime = time.Now()
	}
	p.status = newstat
	p.lastUpdateTime = time.Now()
}

// SetLatency set latency
func (p *Peer) SetLatency(newlat int) {
	p.mux.Lock()
	defer p.mux.Unlock()
	p.latency = newlat
	p.lastUpdateTime = time.Now()
}

// Update lastUpdateTime
func (p *Peer) Update() {
	p.mux.Lock()
	defer p.mux.Unlock()
	p.lastUpdateTime = time.Now()
}

// SetStatusLatency set latency and status
func (p *Peer) SetStatusLatency(newstat, newlat int) {
	p.mux.Lock()
	defer p.mux.Unlock()
	p.latency = newlat
	p.status = newstat
	p.lastUpdateTime = time.Now()
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

type callbackWriteLog func(host string, startTime time.Time, request string, bytessended int, refer, userAgent string, seconds int)

// PeersManager type for managing relays and listeners points
type PeersManager struct {
	mux sync.Mutex
	// peers, who want to be a p2p relay point
	relayPeers peersCollection
	// peers, who want to be a p2p listener
	listenPeers peersCollection
	// links to peers by their ip
	peers map[string]*Peer

	// callback for writing log about peers
	writeLog         callbackWriteLog
	startedInspector int32
	mountName        string
	wg               sync.WaitGroup
}

// Init - init collections, set callback for writing log
func (p *PeersManager) Init(cb callbackWriteLog, mountName string) {
	p.relayPeers = make(peersCollection, 0, 10)
	p.listenPeers = make(peersCollection, 0, 10)
	p.peers = make(map[string]*Peer)
	p.writeLog = cb
	p.mountName = mountName
	atomic.StoreInt32(&p.startedInspector, 1)
	p.wg.Add(1)

	// start inspector
	go p.inspector()
}

// Close ...
func (p *PeersManager) Close() {
	atomic.StoreInt32(&p.startedInspector, 0)
	p.wg.Wait()
}

// watching for peers status
func (p *PeersManager) inspector() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	defer p.wg.Done()

	for {
		if atomic.LoadInt32(&p.startedInspector) == 0 {
			break
		}

		p.mux.Lock()
		for adr, peer := range p.peers {
			// if lastUpdateTime of this peer is more then 10 secs, let consider it closed
			// temp. leave relay alone
			if time.Since(peer.lastUpdateTime) > time.Second*peerLifeTimeUpdateTimeOut {
				if !peer.relay && p.writeLog != nil {
					p.writeLog(adr, peer.connectedTime, "GET /"+p.mountName+" UDP", 0, "-", "penguinClient", int(time.Since(peer.connectedTime).Seconds()))
					p.deletePeer(peer)
				}
				continue
			}
		}
		p.mux.Unlock()

		<-ticker.C
	}

}

// AddOrUpdateRelayPoint add new peer to relays collection or update it's latency
func (p *PeersManager) AddOrUpdateRelayPoint(addr string, latency int) bool {
	p.mux.Lock()
	defer p.mux.Unlock()
	if peer, ok := p.peers[addr]; ok {
		peer.SetLatency(latency)
		return false
	}
	rp := &Peer{addr: addr, relay: true, idx: len(p.relayPeers), latency: latency, lastUpdateTime: time.Now()}
	p.relayPeers = append(p.relayPeers, rp)
	p.peers[addr] = rp
	return true
}

// AddListenPoint add new peer to listener collection
func (p *PeersManager) AddListenPoint(addr string) bool {
	p.mux.Lock()
	defer p.mux.Unlock()
	if _, ok := p.peers[addr]; ok {
		return false
	}
	lp := &Peer{addr: addr, relay: false, idx: len(p.listenPeers), lastUpdateTime: time.Now()}
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
	log.Printf("delete peer %s", item.addr)
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
