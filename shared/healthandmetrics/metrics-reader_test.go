package healthandmetrics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type observable string

func (o observable) MetricsAddress() string {
	return string(o)
}

func Test_httpPrometheusMetricsReader_Read(t *testing.T) {
	server := NewServer("9132", true)
	go server.ListenAndServe()
	defer server.Shutdown(context.Background())

	EnableActiveConnectionsMetrics()
	ActiveConnectionsMetrics.Inc()

	reader := NewHttpPrometheusMetricsReader(time.Second)
	res, err := reader.Read(observable("localhost:9132"))
	assert.NoError(t, err)
	assert.Equal(t, 1, res.ActiveConnections)
}
