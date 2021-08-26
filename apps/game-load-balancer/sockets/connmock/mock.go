package connmock

import (
	"io"
	"math/rand"
	"net"
	"time"
)

type ConnectionMock struct {
	OnWrite func(b []byte) (n int, err error)
	OnRead  func(b []byte) (n int, err error)
	OnClose func() error
}

func (c ConnectionMock) Read(b []byte) (n int, err error) {
	return c.OnRead(b)
}

func (c ConnectionMock) Write(b []byte) (n int, err error) {
	return c.OnWrite(b)
}

func (c ConnectionMock) Close() error {
	return c.OnClose()
}

func (c ConnectionMock) LocalAddr() net.Addr {
	return &net.IPAddr{}
}

func (c ConnectionMock) RemoteAddr() net.Addr {
	return &net.IPAddr{}
}

func (c ConnectionMock) SetDeadline(t time.Time) error {
	return nil
}

func (c ConnectionMock) SetReadDeadline(t time.Time) error {
	return nil
}

func (c ConnectionMock) SetWriteDeadline(t time.Time) error {
	return nil
}

type DataQueue struct {
	Q          [][]byte
	CloseOnEnd bool

	delay time.Duration

	loopedQ [][]byte
	inLoop  bool

	closeChan chan bool

	closed bool

	reachedEnd bool
	i          int
	ii         int
}

func NewDataQueue(closeOnEnd bool, delay time.Duration, q ...[]byte) *DataQueue {
	return &DataQueue{
		Q:          q,
		delay:      delay,
		CloseOnEnd: closeOnEnd,
		closeChan:  make(chan bool, 2),
		reachedEnd: len(q) == 0,
	}
}

func (q *DataQueue) SetLoopedPart(loopedQ ...[]byte) {
	q.loopedQ = loopedQ
}

func (q *DataQueue) OnRead(b []byte) (n int, err error) {
	if q.closed {
		return 0, io.EOF
	}

	if q.reachedEnd {
		if q.CloseOnEnd {
			return 0, io.EOF
		} else if len(q.loopedQ) > 0 {
			q.Q = q.loopedQ
			q.inLoop = true
			q.reachedEnd = false
		} else {
			<-q.closeChan
			return 0, io.EOF
		}
	}

	if q.delay > 0 {
		time.Sleep(q.delay + time.Duration(rand.Intn(int(float64(q.delay)*0.1))))
	}

	var i = 0
	for ; i < len(b); i, q.ii = i+1, q.ii+1 {
		packet := q.Q[q.i]
		if q.ii >= len(packet) {
			q.ii = 0
			if q.i+1 >= len(q.Q) {
				q.reachedEnd = true
				q.ii = 0
				q.i = 0
				return i, nil
			}
			q.i++
			return i, nil
		}

		b[i] = packet[q.ii]
	}

	return i, nil
}

func (q *DataQueue) OnWrite(b []byte) (n int, err error) {
	return len(b), nil
}

func (q *DataQueue) OnClose() error {
	q.closed = true
	q.closeChan <- true
	return nil
}

func (q *DataQueue) Mock() *ConnectionMock {
	return &ConnectionMock{
		OnWrite: q.OnWrite,
		OnRead:  q.OnRead,
		OnClose: q.OnClose,
	}
}
