package middleware

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moderntv/cadre/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func NewMetrics(r *metrics.Registry, subsystem string, metricsAggregation bool) (handler func(*gin.Context), err error) {
	requestsDuration, err := r.RegisterNewSummaryVec(fmt.Sprintf("http_%v_request_duration_us", subsystem), prometheus.SummaryOpts{
		Subsystem: subsystem,

		Name:       "request_duration_us",
		Help:       "The response time of requests",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"endpoint", "status"})
	if err != nil {
		return
	}

	requestsCount, err := r.RegisterNewCounterVec(fmt.Sprintf("http_%v_requests_count", subsystem),
		prometheus.CounterOpts{
			Subsystem: subsystem,

			Name: "requests_total",
			Help: "HTTP requests count",
		},
		[]string{"endpoint", "status"},
	)
	if err != nil {
		return
	}

	handler = func(c *gin.Context) {
		t := time.Now()
		c.Next()
		d := time.Since(t)
		path := c.Request.URL.Path
		if metricsAggregation {
			path = c.FullPath()
		}

		status := strconv.Itoa(c.Writer.Status())
		requestsCount.WithLabelValues(path, status).Inc()
		requestsDuration.WithLabelValues(path, status).Observe(float64(d / time.Microsecond))
	}

	return
}
