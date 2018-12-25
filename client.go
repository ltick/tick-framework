package ltick

import (
	"net"
	"net/http"
	"time"

	"github.com/ltick/tick-framework/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	DEFAULT_DIAL_TIMEOUT            = 3 * time.Second
	DEFAULT_DIAL_KEEP_ALIVE         = 30 * time.Second
	DEFAULT_IDLE_CONN_TIMEOUT       = 90 * time.Second
	DEFAULT_TLS_HANDSHAKE_TIMEOUT   = 2 * time.Second
	DEFAULT_EXPECT_CONTINUE_TIMEOUT = 1 * time.Second
	DEFAULT_MAX_IDLE_CONNS          = 300
	DEFAULT_MAX_IDLE_CONNS_PER_HOST = 100
)

type (
	ClientOption func(*ClientOptions)

	ClientOptions struct {
		Timeout                            time.Duration
		DialTimeout                        time.Duration
		DialKeepAlive                      time.Duration
		IdleConnTimeout                    time.Duration
		TLSHandshakeTimeout                time.Duration
		ExpectContinueTimeout              time.Duration
		MaxIdleConns                       int
		MaxIdleConnsPerHost                int
		MetricsHttpClientRequestsInFlight  prometheus.Gauge
		MetricsHttpClientRequestsCounter   *prometheus.CounterVec
		MetricsHttpClientRequestsDurations []prometheus.ObserverVec
		// observer of get/put connection from idle pool
		MetricsHttpClientRequestsTraceConnection []prometheus.ObserverVec
		// observer of dns
		MetricsHttpClientRequestsTraceDns []prometheus.ObserverVec
		// observer of connect
		MetricsHttpClientRequestsTraceConnect []prometheus.ObserverVec
		// observer of tls handshake
		MetricsHttpClientRequestsTraceTls []prometheus.ObserverVec
		// observer of send request
		MetricsHttpClientRequestsTraceRequest []prometheus.ObserverVec
	}

	Client struct {
		*ClientOptions
	}
)

func ClientTimeout(timeout time.Duration) ClientOption {
	return func(options *ClientOptions) {
		options.Timeout = timeout
	}
}
func ClientDialTimeout(dialTimeout time.Duration) ClientOption {
	return func(options *ClientOptions) {
		options.DialTimeout = dialTimeout
	}
}
func ClientDialKeepAlive(dialKeepAlive time.Duration) ClientOption {
	return func(options *ClientOptions) {
		options.DialKeepAlive = dialKeepAlive
	}
}
func ClientIdleConnTimeout(idleConnTimeout time.Duration) ClientOption {
	return func(options *ClientOptions) {
		options.IdleConnTimeout = idleConnTimeout
	}
}
func ClientTLSHandshakeTimeout(tlsHandshakeTimeout time.Duration) ClientOption {
	return func(options *ClientOptions) {
		options.TLSHandshakeTimeout = tlsHandshakeTimeout
	}
}
func ClientExpectContinueTimeout(expectContinueTimeout time.Duration) ClientOption {
	return func(options *ClientOptions) {
		options.ExpectContinueTimeout = expectContinueTimeout
	}
}
func ClientMaxIdleConns(maxIdleConns int) ClientOption {
	return func(options *ClientOptions) {
		options.MaxIdleConns = maxIdleConns
	}
}
func ClientMaxIdleConnsPerHost(maxIdleConnsPerHost int) ClientOption {
	return func(options *ClientOptions) {
		options.MaxIdleConnsPerHost = maxIdleConnsPerHost
	}
}
func ClientMetricsHttpClientRequestsInFlight(gauge prometheus.Gauge) ClientOption {
	return func(options *ClientOptions) {
		if gauge != nil {
			options.MetricsHttpClientRequestsInFlight = gauge
		} else {
			options.MetricsHttpClientRequestsInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "http_client_requests_in_flight",
				Help: "A gauge of in-flight requests for the wrapped client.",
			})
		}
		prometheus.MustRegister(gauge)
	}
}
func ClientMetricsHttpClientRequestsCounter(counter *prometheus.CounterVec) ClientOption {
	return func(options *ClientOptions) {
		if counter != nil {
			options.MetricsHttpClientRequestsCounter = counter
		} else {
			options.MetricsHttpClientRequestsCounter = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "http_client_requests_total",
					Help: "A counter of requests from the wrapped client.",
				},
				[]string{"server_addr", "host", "method", "path", "status"},
			)
		}
		prometheus.MustRegister(counter)
	}
}
func ClientMetricsHttpClientRequestsDuration(histogram *prometheus.HistogramVec, summary *prometheus.SummaryVec) ClientOption {
	metricsHttpClientRequestsDurations := []prometheus.ObserverVec{}
	if histogram != nil {
		metricsHttpClientRequestsDurations = append(metricsHttpClientRequestsDurations, histogram)
	} else {
		metricsHttpClientRequestsDurations = append(metricsHttpClientRequestsDurations, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_client_requests_duration_seconds",
			Help:    "A histogram of request latencies for requests.",
			Buckets: []float64{.25, .5, 1, 2.5, 5, 10},
		},
			[]string{"server_addr", "host", "method", "path", "status"},
		))
	}
	if summary != nil {
		metricsHttpClientRequestsDurations = append(metricsHttpClientRequestsDurations, summary)
	} else {
		metricsHttpClientRequestsDurations = append(metricsHttpClientRequestsDurations, prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       "http_client_requests_duration_seconds",
				Help:       "A summary of request latencies for requests.",
				Objectives: map[float64]float64{0.9: 0.001, 0.95: 0.001, 0.99: 0.001, 0.999: 0.001, 0.9999: 0.001},
			},
			[]string{"server_addr", "host", "method", "path", "status"},
		))
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsDurations = metricsHttpClientRequestsDurations
		prometheus.MustRegister(histogram, summary)
	}
}

func ClientMetricsHttpClientRequestsTraceConnection(histogram *prometheus.HistogramVec, summary *prometheus.SummaryVec) ClientOption {
	metricsHttpClientRequestsTraceConnection := []prometheus.ObserverVec{}
	if histogram != nil {
		metricsHttpClientRequestsTraceConnection = append(metricsHttpClientRequestsTraceConnection, histogram)
	} else {
		metricsHttpClientRequestsTraceConnection = append(metricsHttpClientRequestsTraceConnection, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_client_requests_connection_duration_seconds",
			Help:    "A histogram of request latencies for connection.",
			Buckets: []float64{.005, .01, .025, .05},
		},
			[]string{"event", "server_addr", "host", "method", "path", "status"},
		))
	}
	if summary != nil {
		metricsHttpClientRequestsTraceConnection = append(metricsHttpClientRequestsTraceConnection, summary)
	} else {
		metricsHttpClientRequestsTraceConnection = append(metricsHttpClientRequestsTraceConnection, prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       "http_client_requests_connection_duration_seconds",
				Help:       "A summary of request latencies for connection.",
				Objectives: map[float64]float64{0.9: 0, 0.95: 0, 0.99: 0, 0.999: 0, 0.9999: 0},
			},
			[]string{"event", "server_addr", "host", "method", "path", "status"},
		))
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsTraceConnection = metricsHttpClientRequestsTraceConnection
		prometheus.MustRegister(histogram, summary)
	}
}

func ClientMetricsHttpClientRequestsTraceDns(histogram *prometheus.HistogramVec, summary *prometheus.SummaryVec) ClientOption {
	metricsHttpClientRequestsTraceDns := []prometheus.ObserverVec{}
	if histogram != nil {
		metricsHttpClientRequestsTraceDns = append(metricsHttpClientRequestsTraceDns, histogram)
	} else {
		metricsHttpClientRequestsTraceDns = append(metricsHttpClientRequestsTraceDns, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_client_requests_dns_duration_seconds",
			Help:    "A histogram of request latencies for dns.",
			Buckets: []float64{.005, .01, .025, .05},
		},
			[]string{"event", "server_addr", "host", "method", "path", "status"},
		))
	}
	if summary != nil {
		metricsHttpClientRequestsTraceDns = append(metricsHttpClientRequestsTraceDns, summary)
	} else {
		metricsHttpClientRequestsTraceDns = append(metricsHttpClientRequestsTraceDns, prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       "http_client_requests_dns_duration_seconds",
				Help:       "A summary of request latencies for dns.",
				Objectives: map[float64]float64{0.9: 0, 0.95: 0, 0.99: 0, 0.999: 0, 0.9999: 0},
			},
			[]string{"event", "server_addr", "host", "method", "path", "status"},
		))
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsTraceDns = metricsHttpClientRequestsTraceDns
		prometheus.MustRegister(histogram, summary)
	}
}

func ClientMetricsHttpClientRequestsTraceConnect(histogram *prometheus.HistogramVec, summary *prometheus.SummaryVec) ClientOption {
	metricsHttpClientRequestsTraceConnect := []prometheus.ObserverVec{}
	if histogram != nil {
		metricsHttpClientRequestsTraceConnect = append(metricsHttpClientRequestsTraceConnect, histogram)
	} else {
		metricsHttpClientRequestsTraceConnect = append(metricsHttpClientRequestsTraceConnect, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_client_requests_connect_duration_seconds",
			Help:    "A histogram of request latencies for connect.",
			Buckets: []float64{.005, .01, .025, .05},
		},
			[]string{"event", "server_addr", "host", "method", "path", "status"},
		))
	}
	if summary != nil {
		metricsHttpClientRequestsTraceConnect = append(metricsHttpClientRequestsTraceConnect, summary)
	} else {
		metricsHttpClientRequestsTraceConnect = append(metricsHttpClientRequestsTraceConnect, prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       "http_client_requests_connect_duration_seconds",
				Help:       "A summary of request latencies for connect.",
				Objectives: map[float64]float64{0.9: 0, 0.95: 0, 0.99: 0, 0.999: 0, 0.9999: 0},
			},
			[]string{"event", "server_addr", "host", "method", "path", "status"},
		))
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsTraceConnect = metricsHttpClientRequestsTraceConnect
		prometheus.MustRegister(histogram, summary)
	}
}

func ClientMetricsHttpClientRequestsTraceTls(histogram *prometheus.HistogramVec, summary *prometheus.SummaryVec) ClientOption {
	metricsHttpClientRequestsTraceTls := []prometheus.ObserverVec{}
	if histogram != nil {
		metricsHttpClientRequestsTraceTls = append(metricsHttpClientRequestsTraceTls, histogram)
	} else {
		metricsHttpClientRequestsTraceTls = append(metricsHttpClientRequestsTraceTls, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_client_requests_tls_duration_seconds",
			Help:    "A histogram of request latencies for tls.",
			Buckets: []float64{.005, .01, .025, .05},
		},
			[]string{"event", "server_addr", "host", "method", "path", "status"},
		))
	}
	if summary != nil {
		metricsHttpClientRequestsTraceTls = append(metricsHttpClientRequestsTraceTls, summary)
	} else {
		metricsHttpClientRequestsTraceTls = append(metricsHttpClientRequestsTraceTls, prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       "http_client_requests_tls_duration_seconds",
				Help:       "A summary of request latencies for tls.",
				Objectives: map[float64]float64{0.9: 0, 0.95: 0, 0.99: 0, 0.999: 0, 0.9999: 0},
			},
			[]string{"event", "server_addr", "host", "method", "path", "status"},
		))
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsTraceTls = metricsHttpClientRequestsTraceTls
		prometheus.MustRegister(histogram, summary)
	}
}

func ClientMetricsHttpClientRequestsTraceRequest(histogram *prometheus.HistogramVec, summary *prometheus.SummaryVec) ClientOption {
	metricsHttpClientRequestsTraceRequest := []prometheus.ObserverVec{}
	if histogram != nil {
		metricsHttpClientRequestsTraceRequest = append(metricsHttpClientRequestsTraceRequest, histogram)
	} else {
		metricsHttpClientRequestsTraceRequest = append(metricsHttpClientRequestsTraceRequest, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_client_requests_request_duration_seconds",
			Help:    "A histogram of request latencies for request.",
			Buckets: []float64{.005, .01, .025, .05},
		},
			[]string{"event", "server_addr", "host", "method", "path", "status"},
		))
	}
	if summary != nil {
		metricsHttpClientRequestsTraceRequest = append(metricsHttpClientRequestsTraceRequest, summary)
	} else {
		metricsHttpClientRequestsTraceRequest = append(metricsHttpClientRequestsTraceRequest, prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       "http_client_requests_request_duration_seconds",
				Help:       "A summary of request latencies for request.",
				Objectives: map[float64]float64{0.9: 0, 0.95: 0, 0.99: 0, 0.999: 0, 0.9999: 0},
			},
			[]string{"event", "server_addr", "host", "method", "path", "status"},
		))
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsTraceRequest = metricsHttpClientRequestsTraceRequest
		prometheus.MustRegister(histogram, summary)
	}
}

func NewHttpClient(setters ...ClientOption) *http.Client {
	c  := Client{
		&ClientOptions{},
	}
	for _, setter := range setters {
		setter(c.ClientOptions)
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = DEFAULT_DIAL_TIMEOUT
	}
	if c.DialKeepAlive == 0 {
		c.DialKeepAlive = DEFAULT_DIAL_KEEP_ALIVE
	}
	if c.IdleConnTimeout == 0 {
		c.IdleConnTimeout = DEFAULT_IDLE_CONN_TIMEOUT
	}
	if c.TLSHandshakeTimeout == 0 {
		c.TLSHandshakeTimeout = DEFAULT_TLS_HANDSHAKE_TIMEOUT
	}
	if c.ExpectContinueTimeout == 0 {
		c.ExpectContinueTimeout = DEFAULT_EXPECT_CONTINUE_TIMEOUT
	}
	if c.ExpectContinueTimeout == 0 {
		c.ExpectContinueTimeout = DEFAULT_EXPECT_CONTINUE_TIMEOUT
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = DEFAULT_MAX_IDLE_CONNS
	}
	if c.MaxIdleConnsPerHost == 0 {
		c.MaxIdleConnsPerHost = DEFAULT_MAX_IDLE_CONNS_PER_HOST
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   c.DialTimeout,
				KeepAlive: c.DialKeepAlive,
			}).DialContext,
			MaxIdleConns:          c.MaxIdleConns,
			IdleConnTimeout:       c.IdleConnTimeout,
			MaxIdleConnsPerHost:   c.MaxIdleConnsPerHost,
			TLSHandshakeTimeout:   c.TLSHandshakeTimeout,
			ExpectContinueTimeout: c.ExpectContinueTimeout,
		},
	}
	if c.Timeout > 0 {
		httpClient.Timeout = c.Timeout
	}
	observers := map[string][]prometheus.ObserverVec{
		"connection": c.MetricsHttpClientRequestsTraceConnection,
		"dns":        c.MetricsHttpClientRequestsTraceDns,
		"connect":    c.MetricsHttpClientRequestsTraceConnect,
		"tls":        c.MetricsHttpClientRequestsTraceTls,
		"request":    c.MetricsHttpClientRequestsTraceRequest,
	}
	// Wrap the default RoundTripper with middleware.
	httpClient.Transport = promhttp.InstrumentRoundTripperInFlight(c.MetricsHttpClientRequestsInFlight,
		metrics.InstrumentHttpClientRoundTripperCounter(c.MetricsHttpClientRequestsCounter,
			metrics.InstrumentHttpClientRoundTripperTrace(observers,
				metrics.InstrumentHttpClientRoundTripperDuration(c.MetricsHttpClientRequestsDurations, httpClient.Transport),
			),
		),
	)
	return httpClient
}
