// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package ice

import "sync"

type PoolManager interface {
	GetPool(size int) (*sync.Pool, error)
	Init(size int) *sync.Pool
}
