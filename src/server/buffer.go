// Package iceserver - icecast streaming server
package iceserver

import (
	"sync"
	"sync/atomic"
)

//BufElement - kind of buffer page
type BufElement struct {
	locked int32
	len    int
	buffer []byte
	pool   *sync.Pool
	next   *BufElement
	prev   *BufElement
	mux    sync.Mutex
}

// BufferQueue - queue, which stores stream fragments from SOURCE
type BufferQueue struct {
	mux           sync.Mutex
	size          int
	maxBufferSize int
	minBufferSize int
	first, last   *BufElement
}

// BufferInfo - struct for monitoring
type BufferInfo struct {
	Size      int
	SizeBytes int
	Graph     string
	InUse     int
}

var pages16kPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 16384)
	},
}

var pages32kPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 32768)
	},
}

// NewBufElement - returns new buffer element (page)
func NewBufElement(buffer []byte, readed int) *BufElement {
	t := &BufElement{}

	if readed >= 32768 {
		t.pool = &pages32kPool
	} else {
		t.pool = &pages16kPool
	}

	t.buffer = t.pool.Get().([]byte)
	t.buffer = t.buffer[:readed]
	t.len = readed
	copy(t.buffer, buffer)
	return t
}

// Reset ...
func (q *BufElement) Reset() {
	q.mux.Lock()
	defer q.mux.Unlock()
	q.len = 0
	q.locked = 0
	q.buffer = q.buffer[:0]
	q.pool.Put(q.buffer)
}

// Next - getting next element
func (q *BufElement) Next() *BufElement {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.next
}

// Lock - mark element as used by listener
func (q *BufElement) Lock() {
	atomic.AddInt32(&q.locked, 1)
}

// UnLock -  mark element as unused by listener
func (q *BufElement) UnLock() {
	atomic.AddInt32(&q.locked, -1)
}

// IsLocked (logical lock, mean it's in use)
func (q *BufElement) IsLocked() bool {
	if atomic.LoadInt32(&q.locked) <= 0 {
		return false
	}
	return true
}

//***************************************

// Init - initiates buffer queue
func (q *BufferQueue) Init(minsize int) {
	q.mux.Lock()
	defer q.mux.Unlock()
	q.size = 0
	q.maxBufferSize = minsize * 3
	q.minBufferSize = minsize
	q.first = nil
	q.last = nil
}

// Size - returns buffer queue size
func (q *BufferQueue) Size() int {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.size
}

// Info - returns buffer state
func (q *BufferQueue) Info() BufferInfo {
	var result BufferInfo
	var t *BufElement
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
func (q *BufferQueue) First() *BufElement {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.first
}

// Last - returns the last element in buffer queue
func (q *BufferQueue) Last() *BufElement {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.last
}

// Start - returns the element to start with
func (q *BufferQueue) Start(burstSize int) *BufElement {
	q.mux.Lock()
	defer q.mux.Unlock()

	burst := 0
	var t *BufElement
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
func (q *BufferQueue) checkAndTruncate() {
	if q.Size() < q.maxBufferSize {
		return
	}

	q.mux.Lock()
	defer q.mux.Unlock()
	var t *BufElement

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
				t.Reset()
				t.next.prev = nil
				t.prev = nil
				q.size--
			} else {
				break
			}
		}
	}
}

// Append - appends new page to the end of the buffer queue
func (q *BufferQueue) Append(buffer []byte, readed int) {
	t := NewBufElement(buffer, readed)

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
