package healthandmetrics

import (
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"
)

type HealthCheckObject interface {
	HealthCheckAddress() string
}

type HealthCheckSuccessObserver func(HealthCheckObject)

type HealthCheckFailedObserver func(HealthCheckObject, error)

type HealthChecker interface {
	AddHealthCheckObject(HealthCheckObject) error
	RemoveHealthCheckObject(HealthCheckObject) error
	AddSuccessObserver(HealthCheckSuccessObserver)
	AddFailedObserver(HealthCheckFailedObserver)
	Start() error
}

type HealthCheckProcessor interface {
	Check(HealthCheckObject) error
}

const defaultHealthCheckFailureThreshold = 2

type HTTPStatusError struct {
	StatusCode int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("bad status code %d", e.StatusCode)
}

type healthCheckResult struct {
	obj HealthCheckObject
	err error
}

type healthChecker struct {
	delay           time.Duration
	processorsCount int

	processor HealthCheckProcessor

	objectsMu sync.RWMutex
	objects   []HealthCheckObject

	observersMu      sync.RWMutex
	successObservers []HealthCheckSuccessObserver
	failedObservers  []HealthCheckFailedObserver

	failureCountsMu  sync.Mutex
	failureCounts    map[string]int
	failureThreshold int

	results chan healthCheckResult
	queue   chan HealthCheckObject
}

func NewHealthChecker(delay time.Duration, processorsCount int, processor HealthCheckProcessor) HealthChecker {
	return &healthChecker{
		delay:            delay,
		processorsCount:  processorsCount,
		processor:        processor,
		failureCounts:    map[string]int{},
		failureThreshold: defaultHealthCheckFailureThreshold,
		results:          make(chan healthCheckResult, 100),
		queue:            make(chan HealthCheckObject, 100),
	}
}

func (h *healthChecker) AddHealthCheckObject(object HealthCheckObject) error {
	h.objectsMu.Lock()
	defer h.objectsMu.Unlock()

	for i, o := range h.objects {
		if o.HealthCheckAddress() == object.HealthCheckAddress() {
			h.objects[i] = object
			h.resetFailureCount(object)
			return nil
		}
	}

	h.objects = append(h.objects, object)
	h.resetFailureCount(object)
	return nil
}

func (h *healthChecker) RemoveHealthCheckObject(object HealthCheckObject) error {
	h.objectsMu.Lock()
	defer h.objectsMu.Unlock()

	for i := range h.objects {
		if h.objects[i].HealthCheckAddress() == object.HealthCheckAddress() {
			if !sameHealthCheckObject(h.objects[i], object) {
				return nil
			}
			h.objects = append(h.objects[:i], h.objects[i+1:]...)
			h.clearFailureCount(object)
			return nil
		}
	}

	h.clearFailureCount(object)
	return nil
}

func (h *healthChecker) AddSuccessObserver(observer HealthCheckSuccessObserver) {
	h.observersMu.Lock()
	defer h.observersMu.Unlock()

	h.successObservers = append(h.successObservers, observer)
}

func (h *healthChecker) AddFailedObserver(observer HealthCheckFailedObserver) {
	h.observersMu.Lock()
	defer h.observersMu.Unlock()

	h.failedObservers = append(h.failedObservers, observer)
}

func (h *healthChecker) Start() error {
	go h.process(h.processorsCount)

	for {
		start := time.Now()
		h.makeIteration()
		waitTime := h.delay - time.Since(start)
		if waitTime > 0 {
			time.Sleep(waitTime)
		}
	}
}

func (h *healthChecker) makeIteration() {
	h.objectsMu.Lock()
	objectsCount := len(h.objects)
	for i := range h.objects {
		h.queue <- h.objects[i]
	}
	h.objectsMu.Unlock()

	if objectsCount == 0 {
		return
	}

	for i := 0; i < objectsCount; i++ {
		result := <-h.results
		h.handleResult(result)
	}
}

func (h *healthChecker) handleResult(result healthCheckResult) {
	if !h.isCurrentHealthCheckObject(result.obj) {
		return
	}

	if result.err != nil {
		if h.recordFailure(result.obj) < h.failureThreshold {
			return
		}

		h.RemoveHealthCheckObject(result.obj)

		h.observersMu.RLock()
		defer h.observersMu.RUnlock()

		for _, observer := range h.failedObservers {
			observer(result.obj, result.err)
		}
	} else {
		h.resetFailureCount(result.obj)

		h.observersMu.RLock()
		defer h.observersMu.RUnlock()

		for _, observer := range h.successObservers {
			observer(result.obj)
		}
	}
}

func (h *healthChecker) isCurrentHealthCheckObject(object HealthCheckObject) bool {
	h.objectsMu.RLock()
	defer h.objectsMu.RUnlock()

	for _, current := range h.objects {
		if current.HealthCheckAddress() != object.HealthCheckAddress() {
			continue
		}

		return sameHealthCheckObject(current, object)
	}

	return false
}

func sameHealthCheckObject(a, b HealthCheckObject) bool {
	if a == nil || b == nil {
		return a == b
	}

	aType := reflect.TypeOf(a)
	if aType != reflect.TypeOf(b) || !aType.Comparable() {
		return true
	}

	return a == b
}

func (h *healthChecker) recordFailure(object HealthCheckObject) int {
	h.failureCountsMu.Lock()
	defer h.failureCountsMu.Unlock()

	address := object.HealthCheckAddress()
	h.failureCounts[address]++
	return h.failureCounts[address]
}

func (h *healthChecker) resetFailureCount(object HealthCheckObject) {
	h.failureCountsMu.Lock()
	defer h.failureCountsMu.Unlock()

	h.failureCounts[object.HealthCheckAddress()] = 0
}

func (h *healthChecker) clearFailureCount(object HealthCheckObject) {
	h.failureCountsMu.Lock()
	defer h.failureCountsMu.Unlock()

	delete(h.failureCounts, object.HealthCheckAddress())
}

func (h *healthChecker) process(processorsCount int) {
	for i := 0; i < processorsCount; i++ {
		go func() {
			for obj := range h.queue {
				h.results <- healthCheckResult{
					obj: obj,
					err: h.processor.Check(obj),
				}
			}
		}()
	}
}

type httpHealthCheckProcessor struct {
	client *http.Client
}

func NewHttpHealthCheckProcessor(timeout time.Duration) HealthCheckProcessor {
	return &httpHealthCheckProcessor{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (h *httpHealthCheckProcessor) Check(object HealthCheckObject) error {
	resp, err := h.client.Get("http://" + object.HealthCheckAddress() + HealthCheckURL)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &HTTPStatusError{StatusCode: resp.StatusCode}
	}

	return nil
}
