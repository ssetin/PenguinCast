package ice

import "sync"

type PoolManager interface {
	GetPool(size int) (*sync.Pool, error)
	Init(size int) *sync.Pool
}
