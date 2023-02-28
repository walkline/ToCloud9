package healthandmetrics

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type healthCheckObj string

func (h healthCheckObj) HealthCheckAddress() string {
	return string(h)
}

type healthCheckProcessorMock struct {
	err        error
	delay      time.Duration
	checkCount int
	m          sync.Mutex
}

func (h *healthCheckProcessorMock) Check(object HealthCheckObject) error {
	if h.delay > 0 {
		time.Sleep(h.delay)
	}
	h.m.Lock()
	h.checkCount++
	h.m.Unlock()
	return h.err
}

func (h *healthCheckProcessorMock) checksCount() int {
	h.m.Lock()
	defer h.m.Unlock()
	return h.checkCount
}

func Test_healthChecker_StartSkippingDelay(t *testing.T) {
	proc := healthCheckProcessorMock{
		err:   nil,
		delay: time.Millisecond * 5,
	}

	obj := healthCheckObj("127.0.0.1:9000")
	checker := NewHealthChecker(time.Millisecond*4, 1, &proc)
	checker.AddHealthCheckObject(obj)
	go checker.Start()

	time.Sleep(time.Millisecond * 13)

	assert.Equal(t, 2, proc.checksCount())
}

func Test_healthChecker_StartRespectDelay(t *testing.T) {
	proc := healthCheckProcessorMock{
		err:   nil,
		delay: time.Millisecond * 5,
	}

	obj := healthCheckObj("127.0.0.1:9000")
	checker := NewHealthChecker(time.Millisecond*10, 1, &proc)
	checker.AddHealthCheckObject(obj)
	go checker.Start()

	time.Sleep(time.Millisecond * 13)

	assert.Equal(t, 1, proc.checksCount())
}

func Test_healthChecker_StartWithAddingObjectsAfterStart(t *testing.T) {
	proc := healthCheckProcessorMock{
		err:   nil,
		delay: time.Millisecond * 1,
	}

	obj := healthCheckObj("127.0.0.1:9000")
	checker := NewHealthChecker(time.Millisecond*5, 1, &proc)
	go checker.Start()
	time.Sleep(time.Millisecond * 1)

	checker.AddHealthCheckObject(obj)
	time.Sleep(time.Millisecond * 8)

	assert.Equal(t, 1, proc.checksCount())
}
