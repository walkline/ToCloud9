package healthandmetrics

import (
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
}

func (h *healthCheckProcessorMock) Check(object HealthCheckObject) error {
	if h.delay > 0 {
		time.Sleep(h.delay)
	}
	h.checkCount++
	return h.err
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

	assert.Equal(t, 2, proc.checkCount)
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

	assert.Equal(t, 1, proc.checkCount)
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

	assert.Equal(t, 1, proc.checkCount)
}
