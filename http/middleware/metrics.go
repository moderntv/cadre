package middleware

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/moderntv/cadre/metrics"
)

func NewMetrics(r *metrics.Registry, subsystem string) (handler func(*gin.Context), err error) {
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

		go func(c *gin.Context, d time.Duration) {
			path := c.Request.URL.Path
			for _, param := range c.Params {
				path = strings.ReplaceAll(path, param.Value, fmt.Sprintf(":%s", param.Key))
			}

			requestsCount.WithLabelValues(path, fmt.Sprintf("%v", c.Writer.Status())).Inc()
			requestsDuration.WithLabelValues(path, fmt.Sprintf("%v", c.Writer.Status())).Observe(float64(d / time.Microsecond))
		}(c.Copy(), d)
	}

	return
}
