package healthandmetrics

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog/log"
)

type MetricsObservable interface {
	MetricsAddress() string
}

type MetricsRead struct {
	ActiveConnections int

	DelayMean         int
	DelayMedian       int
	Delay95Percentile int
	Delay99Percentile int
	DelayMax          int

	Raw []dto.MetricFamily
}

type MetricsObserver func(MetricsObservable, *MetricsRead)

type MetricsConsumer interface {
	AddMetricsObservable(MetricsObservable) error
	RemoveMetricsObservable(MetricsObservable) error
	AddObserver(MetricsObserver)
	Start() error
}

type MetricsReader interface {
	Read(MetricsObservable) (*MetricsRead, error)
}

type metricsReadResult struct {
	Observable  MetricsObservable
	MetricsRead *MetricsRead
	Err         error
}

type metricsConsumerImpl struct {
	delay           time.Duration
	processorsCount int

	reader MetricsReader

	objectsMu sync.RWMutex
	objects   []MetricsObservable

	observersMu sync.RWMutex
	observers   []MetricsObserver

	results chan metricsReadResult
	queue   chan MetricsObservable
}

func (m *metricsConsumerImpl) AddMetricsObservable(observable MetricsObservable) error {
	m.objectsMu.Lock()
	defer m.objectsMu.Unlock()

	m.objects = append(m.objects, observable)
	return nil
}

func (m *metricsConsumerImpl) RemoveMetricsObservable(observable MetricsObservable) error {
	m.objectsMu.Lock()
	defer m.objectsMu.Unlock()

	for i := range m.objects {
		if m.objects[i].MetricsAddress() == observable.MetricsAddress() {
			m.objects = append(m.objects[:i], m.objects[i+1:]...)
			return nil
		}
	}

	return nil
}

func (m *metricsConsumerImpl) AddObserver(observer MetricsObserver) {
	m.observersMu.Lock()
	defer m.observersMu.Unlock()

	m.observers = append(m.observers, observer)
}

func (m *metricsConsumerImpl) Start() error {
	go m.process(m.processorsCount)

	for {
		start := time.Now()
		m.makeIteration()
		waitTime := m.delay - time.Since(start)
		if waitTime > 0 {
			time.Sleep(waitTime)
		}
	}
}

func (m *metricsConsumerImpl) process(processorsCount int) {
	for i := 0; i < processorsCount; i++ {
		go func() {
			for obj := range m.queue {
				metrics, err := m.reader.Read(obj)
				m.results <- metricsReadResult{
					Observable:  obj,
					MetricsRead: metrics,
					Err:         err,
				}
			}
		}()
	}
}

func (m *metricsConsumerImpl) makeIteration() {
	m.objectsMu.Lock()
	objectsCount := len(m.objects)
	for i := range m.objects {
		m.queue <- m.objects[i]
	}
	m.objectsMu.Unlock()

	if objectsCount == 0 {
		return
	}

	for i := 0; i < objectsCount; i++ {
		result := <-m.results
		m.handleResult(result)
	}
}

func (m *metricsConsumerImpl) handleResult(result metricsReadResult) {
	if result.Err != nil {
		log.Error().Err(result.Err).Msgf("failed to read metrics for %s object", result.Observable.MetricsAddress())
		return
	}

	m.observersMu.RLock()
	defer m.observersMu.RUnlock()

	for _, observer := range m.observers {
		observer(result.Observable, result.MetricsRead)
	}
}

func NewMetricsConsumer(delay time.Duration, processorsCount int, reader MetricsReader) MetricsConsumer {
	return &metricsConsumerImpl{
		delay:           delay,
		processorsCount: processorsCount,
		reader:          reader,
		results:         make(chan metricsReadResult, 100),
		queue:           make(chan MetricsObservable, 100),
	}
}

type httpPrometheusMetricsReader struct {
	client *http.Client
}

func NewHttpPrometheusMetricsReader(timeout time.Duration) MetricsReader {
	return &httpPrometheusMetricsReader{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (h httpPrometheusMetricsReader) Read(observable MetricsObservable) (*MetricsRead, error) {
	resp, err := h.client.Get("http://" + observable.MetricsAddress() + MetricsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code %d", resp.StatusCode)
	}

	metrics := []dto.MetricFamily{}
	dec := expfmt.NewDecoder(resp.Body, expfmt.NewFormat(expfmt.TypeTextPlain))

	for {
		result := dto.MetricFamily{}
		err = dec.Decode(&result)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, result)
	}

	results := MetricsRead{
		Raw: metrics,
	}
	for _, result := range metrics {
		if result.Name == nil || len(result.Metric) == 0 {
			continue
		}

		switch *result.Name {
		case activeConnectionMetricsName:
			results.ActiveConnections = int(*result.Metric[0].Gauge.Value)
		case delayMeanMetricsName:
			results.DelayMean = int(*result.Metric[0].Gauge.Value)
		case delayMedianMetricsName:
			results.DelayMedian = int(*result.Metric[0].Gauge.Value)
		case delay95PercentileMetricsName:
			results.Delay95Percentile = int(*result.Metric[0].Gauge.Value)
		case delay99PercentileMetricsName:
			results.Delay99Percentile = int(*result.Metric[0].Gauge.Value)
		case delayMaxMetricsName:
			results.DelayMax = int(*result.Metric[0].Gauge.Value)
		}
	}

	return &results, nil
}
