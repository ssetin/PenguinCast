// Package iceserver - icecast streaming server
package iceserver

import (
	"errors"
	"sync"
)

// PoolManager ...
type PoolManager struct {
	pages map[int]*sync.Pool
}

// Init ...
func (p *PoolManager) Init(size int) *sync.Pool {

	if p.pages == nil {
		p.pages = make(map[int]*sync.Pool)
	}

	if pool, ok := p.pages[size]; ok {
		return pool
	}

	p.pages[size] = &sync.Pool{
		New: func() interface{} {
			return make([]byte, size)
		},
	}

	return p.pages[size]
}

// GetPool ...
func (p *PoolManager) GetPool(size int) (*sync.Pool, error) {
	if pool, ok := p.pages[size]; ok {
		return pool, nil
	}
	return nil, errors.New("No pool according to that size. You have to initialize it")
}
