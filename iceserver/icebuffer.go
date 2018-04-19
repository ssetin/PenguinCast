package iceserver

/*
	TODO:
	- manage buffer size
*/

import (
	"log"
	"sync"
)

//BufElement ...
type BufElement struct {
	locked bool
	buffer []byte
	next   *BufElement
	mux    sync.Mutex
}

// BufferQueue ...
type BufferQueue struct {
	mux           sync.Mutex
	size          int
	maxBufferSize int
	first, last   *BufElement
}

// NewbufElement ...
func NewbufElement() *BufElement {
	var t *BufElement
	t = &BufElement{locked: false}
	return t
}

//Next ...
func (q *BufElement) Next() *BufElement {
	//q.mux.Lock()
	//defer q.mux.Unlock()
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

//IsLocked ...
func (q *BufElement) IsLocked() bool {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.locked
}

//***************************************

//Init ...
func (q *BufferQueue) Init(maxsize int) {
	q.mux.Lock()
	defer q.mux.Unlock()
	q.size = 0
	q.maxBufferSize = maxsize
	q.first = nil
	q.last = nil
}

//Size ...
func (q *BufferQueue) Size() int {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.size
}

//Print ...
func (q *BufferQueue) Print() {
	q.mux.Lock()
	defer q.mux.Unlock()
	var t *BufElement
	t = q.first
	str := ""

	for {
		if t == nil {
			break
		}
		if t.IsLocked() {
			str = str + "1"
		} else {
			str = str + "0"
		}
		t = t.next
	}
	log.Println("Buffer: " + str)
}

//First ...
func (q *BufferQueue) First() *BufElement {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.first
}

//Last ...
func (q *BufferQueue) Last() *BufElement {
	q.mux.Lock()
	defer q.mux.Unlock()
	return q.last
}

//checkAndTruncate ...
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
				q.first = t.next
				q.size--
			} else {
				break
			}
		}
		t = t.next
	}

}

//Append ...
func (q *BufferQueue) Append(buffer []byte, readed int) {
	var t *BufElement
	t = NewbufElement()
	t.buffer = buffer[:readed]

	q.mux.Lock()
	defer q.mux.Unlock()

	if q.size == 0 {
		q.size = 1
		t.next = nil
		q.first = t
		q.last = t
		return
	}

	q.last.next = t
	q.last = t
	q.size++
}
