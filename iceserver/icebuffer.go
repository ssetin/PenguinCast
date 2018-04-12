package iceserver

/*
	TODO:
	- manage buffer size
*/

import "sync"

//BufElement ...
type BufElement struct {
	iam    int /*temporary*/
	used   int
	buffer []byte
	next   *BufElement
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
	t = &BufElement{used: 0}
	return t
}

//Next ...
func (q *BufElement) Next() *BufElement {
	q.used++
	return q.next
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
		t.iam = 1
		q.first = t
		q.last = t
		return
	}

	t.iam = q.size + 1
	q.last.next = t
	q.last = t
	q.size++
}
