// Package iceserver - icecast streaming server
package iceserver

import (
	"sync"
)

//BufElement - kind of buffer page
type BufElement struct {
	locked bool
	buffer []byte
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

// NewbufElement - returns new buffer element (page)
func NewbufElement() *BufElement {
	var t *BufElement
	t = &BufElement{locked: false}
	return t
}

//Next ...
func (q *BufElement) Next() *BufElement {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.next
}

//Lock ...
func (q *BufElement) Lock() {
	q.mux.Lock()
	defer q.mux.Unlock()
	q.locked = true
}

//UnLock ...
func (q *BufElement) UnLock() {
	q.mux.Lock()
	defer q.mux.Unlock()
	q.locked = false
}

//IsLocked (logical lock, mean it's in use)
func (q *BufElement) IsLocked() bool {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.locked
}

//***************************************

//Init - initiates buffer queue
func (q *BufferQueue) Init(maxsize int) {
	q.mux.Lock()
	defer q.mux.Unlock()
	q.size = 0
	q.maxBufferSize = maxsize
	q.minBufferSize = 6
	q.first = nil
	q.last = nil
}

//Size - returns buffer queue size
func (q *BufferQueue) Size() int {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.size
}

// Info - returns buffer state
func (q *BufferQueue) Info() BufferInfo {
	var result BufferInfo
	q.mux.Lock()
	defer q.mux.Unlock()
	var t *BufElement
	t = q.first
	str := ""

	for {
		if t == nil {
			break
		}
		result.Size++
		result.SizeBytes += len(t.buffer)

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

//First - returns the first element in buffer queue
func (q *BufferQueue) First() *BufElement {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.first
}

//Last - returns the last element in buffer queue
func (q *BufferQueue) Last() *BufElement {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.last
}

//Start - returns the element to start with
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
		if t.prev == nil || burst >= burstSize {
			break
		}
		burst += len(t.buffer)
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
	t = q.first

	for {
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
				t.next.prev = nil
				q.first = t.next
				q.size--
			} else {
				break
			}
		}
		t = t.next
	}

}

//Append - appends new page to the end of the buffer queue
func (q *BufferQueue) Append(buffer []byte, readed int) {
	var t *BufElement
	t = NewbufElement()
	//t.buffer = buffer[:readed]
	t.buffer = make([]byte, readed)
	copy(t.buffer, buffer)

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
