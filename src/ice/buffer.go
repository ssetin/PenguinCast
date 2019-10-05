// Copyright 2019 Setin Sergei
// Licensed under the Apache License, Version 2.0 (the "License")

package ice

import (
	"sync"
	"sync/atomic"
)

//BufElement - kind of buffer page
type bufElement struct {
	locked int32
	len    int
	buffer []byte
	next   *bufElement
	prev   *bufElement
	mux    sync.Mutex
}

// BufferQueue - queue, which stores stream fragments from SOURCE
type bufferQueue struct {
	mux           sync.Mutex
	size          int
	maxBufferSize int
	minBufferSize int
	first, last   *bufElement
	pool          *sync.Pool
}

// BufferInfo - struct for monitoring
type bufferInfo struct {
	Size      int
	SizeBytes int
	Graph     string
	InUse     int
}

// Reset ...
func (q *bufElement) Reset(pool *sync.Pool) {
	q.mux.Lock()
	defer q.mux.Unlock()
	q.len = 0
	q.locked = 0
	if q.next != nil {
		q.next.prev = nil
		q.next = nil
	}
	if q.prev != nil {
		q.prev.next = nil
		q.prev = nil
	}

	q.buffer = q.buffer[:0]
	pool.Put(q.buffer)
}

// Next - getting next element
func (q *bufElement) Next() *bufElement {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.next
}

// Lock - mark element as used by listener
func (q *bufElement) Lock() {
	atomic.AddInt32(&q.locked, 1)
}

// UnLock -  mark element as unused by listener
func (q *bufElement) UnLock() {
	atomic.AddInt32(&q.locked, -1)
}

// IsLocked (logical lock, mean it's in use)
func (q *bufElement) IsLocked() bool {
	if atomic.LoadInt32(&q.locked) <= 0 {
		return false
	}
	return true
}

//***************************************

// Init - initiates buffer queue
func (q *bufferQueue) Init(minSize int, pool *sync.Pool) {
	q.mux.Lock()
	defer q.mux.Unlock()
	q.size = 0
	q.maxBufferSize = minSize * 8
	q.minBufferSize = minSize
	q.first = nil
	q.last = nil
	q.pool = pool
}

// NewBufElement - returns new buffer element (page)
func (q *bufferQueue) newBufElement(buffer []byte, readed int) *bufElement {
	t := &bufElement{}

	if q.pool == nil {
		return nil
	}

	t.buffer = q.pool.Get().([]byte)
	t.buffer = t.buffer[:readed]
	t.len = readed
	copy(t.buffer, buffer)
	return t
}

// Size - returns buffer queue size
func (q *bufferQueue) Size() int {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.size
}

// Info - returns buffer state
func (q *bufferQueue) Info() bufferInfo {
	var result bufferInfo
	var t *bufElement
	str := ""

	q.mux.Lock()
	defer q.mux.Unlock()

	t = q.first

	for {
		if t == nil {
			break
		}
		result.Size++
		result.SizeBytes += t.len

		if t.IsLocked() {
			str = str + "1"
			result.InUse++
		} else {
			str = str + "0"
		}
		t = t.next
	}

	result.Graph = str

	return result
}

// First - returns the first element in buffer queue
func (q *bufferQueue) First() *bufElement {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.first
}

// Last - returns the last element in buffer queue
func (q *bufferQueue) Last() *bufElement {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.last
}

// Start - returns the element to start with
func (q *bufferQueue) Start(burstSize int) *bufElement {
	q.mux.Lock()
	defer q.mux.Unlock()

	burst := 0
	var t *bufElement
	t = q.last
	if t == nil {
		return nil
	}

	for {
		if t.prev == nil || burst > burstSize {
			break
		}
		burst += t.len
		t = t.prev
	}
	return t
}

// checkAndTruncate - check if the max buffer size is reached and try to truncate it
// taking into account pages, which still in use
func (q *bufferQueue) checkAndTruncate() {
	if q.Size() < q.maxBufferSize {
		return
	}

	q.mux.Lock()
	defer q.mux.Unlock()
	var t *bufElement

	for {
		t = q.first
		if t == nil || q.size <= 1 {
			break
		}
		if t.IsLocked() {
			break
		} else {
			if t.next != nil {
				if q.size <= q.minBufferSize {
					break
				}
				q.first = t.next
				t.Reset(q.pool)
				t = nil
				q.size--
			} else {
				break
			}
		}
	}
}

// Append - appends new page to the end of the buffer queue
func (q *bufferQueue) Append(buffer []byte, read int) {
	t := q.newBufElement(buffer, read)
	if t == nil {
		return
	}

	q.mux.Lock()
	defer q.mux.Unlock()

	if q.size == 0 {
		q.size = 1
		t.next = nil
		t.prev = nil
		q.first = t
		q.last = t
		return
	}

	q.last.mux.Lock()
	q.last.next = t
	t.prev = q.last
	q.last.mux.Unlock()
	q.last = t
	q.size++
}
