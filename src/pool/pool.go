// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package pool

import (
	"errors"
	"sync"
)

// Manager ...
type manager struct {
	pages map[int]*sync.Pool
}

func NewPoolManager() *manager {
	return &manager{}
}

// Init ...
func (p *manager) Init(size int) *sync.Pool {

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
func (p *manager) GetPool(size int) (*sync.Pool, error) {
	if pool, ok := p.pages[size]; ok {
		return pool, nil
	}
	return nil, errors.New("no pool according to that size. You have to initialize it")
}
