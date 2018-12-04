package middleware

import (
	"context"
	"net/http"

	"github.com/ltick/tick-routing"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// apiRequestDuration tracks the duration separate for each HTTP status
	// class (1xx, 2xx, ...). This creates a fair amount of time series on
	// the Prometheus server. Usually, you would track the duration of
	// serving HTTP request without partitioning by outcome. Do something
	// like this only if needed. Also note how only status classes are
	// tracked, not every single status code. The latter would create an
	// even larger amount of time series. Request counters partitioned by
	// status code are usually OK as each counter only creates one time
	// series. Histograms are way more expensive, so partition with care and
	// only where you really need separate latency tracking. Partitioning by
	// status class is only an example. In concrete cases, other partitions
	// might make more sense.
	apiRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_request_duration_seconds",
			Help:    "Histogram for the request duration of the public API, partitioned by status class.",
			Buckets: prometheus.ExponentialBuckets(0.1, 1.5, 5),
		},
		[]string{"status_class"},
	)
)

type Prometheus struct {
	timer *prometheus.Timer
}

// NewPrometheus creates middleware that intercepts the specified IP prefix.
func NewPrometheus() *Prometheus {
	return &Prometheus{}
}
func (i *Prometheus) Prepare(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (i *Prometheus) Initiate(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (i *Prometheus) OnRequestStartup(c *routing.Context) error {
	status := http.StatusOK
	// The ObserverFunc gets called by the deferred ObserveDuration and
	// decides which Histogram's Observe method is called.
	i.timer = prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		switch {
		case status >= 500: // Server error.
			apiRequestDuration.WithLabelValues("5xx").Observe(v)
		case status >= 400: // Client error.
			apiRequestDuration.WithLabelValues("4xx").Observe(v)
		case status >= 300: // Redirection.
			apiRequestDuration.WithLabelValues("3xx").Observe(v)
		case status >= 200: // Success.
			apiRequestDuration.WithLabelValues("2xx").Observe(v)
		default: // Informational.
			apiRequestDuration.WithLabelValues("1xx").Observe(v)
		}
	}))
	return nil
}

func (i *Prometheus) OnRequestShutdown(c *routing.Context) error {
	i.timer.ObserveDuration()
	return nil
}
