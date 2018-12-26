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
		MetricsHttpClientRequestLabelFunc     metrics.HttpClientRequestLabelFunc
	}

	Client struct {
		*ClientOptions
	}
)

var (
	defaultHttpClientRequestsInFlight                 prometheus.Gauge
	defaultHttpClientRequestsCounter                  *prometheus.CounterVec
	defaultHttpClientRequestsDurationHistogram        *prometheus.HistogramVec
	defaultHttpClientRequestsDurationSummary          *prometheus.SummaryVec
	defaultHttpClientRequestsTraceConnectionHistogram *prometheus.HistogramVec
	defaultHttpClientRequestsTraceConnectionSummary   *prometheus.SummaryVec
	defaultHttpClientRequestsTraceConnectHistogram    *prometheus.HistogramVec
	defaultHttpClientRequestsTraceConnectSummary      *prometheus.SummaryVec
	defaultHttpClientRequestsTraceDnsHistogram        *prometheus.HistogramVec
	defaultHttpClientRequestsTraceDnsSummary          *prometheus.SummaryVec
	defaultHttpClientRequestsTraceTlsHistogram        *prometheus.HistogramVec
	defaultHttpClientRequestsTraceTlsSummary          *prometheus.SummaryVec
	defaultHttpClientRequestsTraceRequestHistogram    *prometheus.HistogramVec
	defaultHttpClientRequestsTraceRequestSummary      *prometheus.SummaryVec
)

func init() {
	defaultHttpClientRequestsInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_client_requests_in_flight",
		Help: "A gauge of in-flight requests for the wrapped client.",
	})
	prometheus.MustRegister(defaultHttpClientRequestsInFlight)
	defaultHttpClientRequestsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_client_requests_total",
			Help: "A counter of requests from the wrapped client.",
		},
		[]string{"server_addr", "host", "method", "path", "status"},
	)
	prometheus.MustRegister(defaultHttpClientRequestsCounter)
	defaultHttpClientRequestsDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_duration_seconds",
		Help:    "A histogram of request latencies for requests.",
		Buckets: []float64{.25, .5, 1, 2.5, 5, 10},
	},
		[]string{"server_addr", "host", "method", "path", "status"},
	)
	prometheus.MustRegister(defaultHttpClientRequestsDurationHistogram)
	defaultHttpClientRequestsDurationSummary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "http_client_requests_duration_seconds_summary",
			Help:       "A summary of request latencies for requests.",
			Objectives: map[float64]float64{0.9: 0.001, 0.95: 0.001, 0.99: 0.001, 0.999: 0.001, 0.9999: 0.001},
		},
		[]string{"server_addr", "host", "method", "path", "status"},
	)
	prometheus.MustRegister(defaultHttpClientRequestsDurationSummary)
	defaultHttpClientRequestsTraceConnectionHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_connection_duration_seconds",
		Help:    "A histogram of request latencies for connection.",
		Buckets: []float64{.005, .01, .025, .05},
	},
		[]string{"event", "server_addr", "host", "method", "path"},
	)
	prometheus.MustRegister(defaultHttpClientRequestsTraceConnectionHistogram)
	defaultHttpClientRequestsTraceConnectionSummary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "http_client_requests_connection_duration_seconds_summary",
			Help:       "A summary of request latencies for connection.",
			Objectives: map[float64]float64{0.9: 0, 0.95: 0, 0.99: 0, 0.999: 0, 0.9999: 0},
		},
		[]string{"event", "server_addr", "host", "method", "path"},
	)
	defaultHttpClientRequestsTraceConnectHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_connect_duration_seconds",
		Help:    "A histogram of request latencies for connect.",
		Buckets: []float64{.005, .01, .025, .05},
	},
		[]string{"event", "server_addr", "host", "method", "path"},
	)
	defaultHttpClientRequestsTraceConnectSummary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "http_client_requests_connect_duration_seconds_summary",
			Help:       "A summary of request latencies for connect.",
			Objectives: map[float64]float64{0.9: 0, 0.95: 0, 0.99: 0, 0.999: 0, 0.9999: 0},
		},
		[]string{"event", "server_addr", "host", "method", "path"},
	)
	prometheus.MustRegister(defaultHttpClientRequestsTraceConnectionSummary)
	defaultHttpClientRequestsTraceDnsHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_dns_duration_seconds",
		Help:    "A histogram of request latencies for dns.",
		Buckets: []float64{.005, .01, .025, .05},
	},
		[]string{"event", "server_addr", "host", "method", "path"},
	)
	prometheus.MustRegister(defaultHttpClientRequestsTraceDnsHistogram)

	defaultHttpClientRequestsTraceDnsSummary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "http_client_requests_dns_duration_seconds_summary",
			Help:       "A summary of request latencies for dns.",
			Objectives: map[float64]float64{0.9: 0, 0.95: 0, 0.99: 0, 0.999: 0, 0.9999: 0},
		},
		[]string{"event", "server_addr", "host", "method", "path"},
	)
	prometheus.MustRegister(defaultHttpClientRequestsTraceDnsSummary)

	defaultHttpClientRequestsTraceTlsHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_tls_duration_seconds",
		Help:    "A histogram of request latencies for tls.",
		Buckets: []float64{.005, .01, .025, .05},
	},
		[]string{"event", "server_addr", "host", "method", "path"},
	)

	defaultHttpClientRequestsTraceTlsSummary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "http_client_requests_tls_duration_seconds_summary",
			Help:       "A summary of request latencies for tls.",
			Objectives: map[float64]float64{0.9: 0, 0.95: 0, 0.99: 0, 0.999: 0, 0.9999: 0},
		},
		[]string{"event", "server_addr", "host", "method", "path"},
	)

	defaultHttpClientRequestsTraceRequestHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_request_duration_seconds",
		Help:    "A histogram of request latencies for request.",
		Buckets: []float64{.005, .01, .025, .05},
	},
		[]string{"event", "server_addr", "host", "method", "path"},
	)

	defaultHttpClientRequestsTraceRequestSummary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "http_client_requests_request_duration_seconds_summary",
			Help:       "A summary of request latencies for request.",
			Objectives: map[float64]float64{0.9: 0, 0.95: 0, 0.99: 0, 0.999: 0, 0.9999: 0},
		},
		[]string{"event", "server_addr", "host", "method", "path"},
	)

}

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
			options.MetricsHttpClientRequestsInFlight = defaultHttpClientRequestsInFlight
		}

	}
}
func ClientMetricsHttpClientRequestsCounter(counter *prometheus.CounterVec) ClientOption {
	return func(options *ClientOptions) {
		if counter != nil {
			options.MetricsHttpClientRequestsCounter = counter
		} else {
			options.MetricsHttpClientRequestsCounter = defaultHttpClientRequestsCounter
		}
	}
}
func ClientMetricsHttpClientRequestsDuration(histogram *prometheus.HistogramVec, summary *prometheus.SummaryVec) ClientOption {
	if histogram == nil {
		histogram = defaultHttpClientRequestsDurationHistogram
	}
	if summary == nil {
		summary = defaultHttpClientRequestsDurationSummary
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsDurations = []prometheus.ObserverVec{histogram, summary}
	}
}
func ClientMetricsHttpClientRequestsTraceConnection(histogram *prometheus.HistogramVec, summary *prometheus.SummaryVec) ClientOption {
	if histogram == nil {
		histogram = defaultHttpClientRequestsTraceConnectionHistogram
	}
	if summary == nil {
		summary = defaultHttpClientRequestsTraceConnectionSummary
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsTraceConnection = []prometheus.ObserverVec{histogram, summary}
	}
}
func ClientMetricsHttpClientRequestsTraceDns(histogram *prometheus.HistogramVec, summary *prometheus.SummaryVec) ClientOption {
	if histogram == nil {
		histogram = defaultHttpClientRequestsTraceDnsHistogram
	}
	if summary == nil {
		summary = defaultHttpClientRequestsTraceDnsSummary
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsTraceDns = []prometheus.ObserverVec{histogram, summary}
	}
}
func ClientMetricsHttpClientRequestsTraceConnect(histogram *prometheus.HistogramVec, summary *prometheus.SummaryVec) ClientOption {
	if histogram == nil {
		histogram = defaultHttpClientRequestsTraceConnectHistogram
	}
	if summary == nil {
		summary = defaultHttpClientRequestsTraceConnectSummary
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsTraceConnect = []prometheus.ObserverVec{histogram, summary}
	}
}
func ClientMetricsHttpClientRequestsTraceTls(histogram *prometheus.HistogramVec, summary *prometheus.SummaryVec) ClientOption {
	if histogram == nil {
		histogram = defaultHttpClientRequestsTraceTlsHistogram
	}
	if summary == nil {
		summary = defaultHttpClientRequestsTraceTlsSummary
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsTraceTls = []prometheus.ObserverVec{histogram, summary}
	}
}
func ClientMetricsHttpClientRequestsTraceRequest(histogram *prometheus.HistogramVec, summary *prometheus.SummaryVec) ClientOption {
	if histogram == nil {
		histogram = defaultHttpClientRequestsTraceRequestHistogram
	}
	if summary == nil {
		summary = defaultHttpClientRequestsTraceRequestSummary
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsTraceRequest = []prometheus.ObserverVec{histogram, summary}
	}
}
func ClientMetricsHttpClientRequestLabelFunc(httpClientRequestLabelFunc metrics.HttpClientRequestLabelFunc) ClientOption {
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestLabelFunc = httpClientRequestLabelFunc
	}
}
func NewHttpClient(setters ...ClientOption) *http.Client {
	c := Client{
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

	// Wrap the default RoundTripper with middleware.
	if c.MetricsHttpClientRequestsDurations != nil {
		if c.MetricsHttpClientRequestLabelFunc != nil {
			httpClient.Transport = metrics.InstrumentHttpClientRequestDuration(c.MetricsHttpClientRequestsDurations, httpClient.Transport, c.MetricsHttpClientRequestLabelFunc)
		} else {
			httpClient.Transport = metrics.InstrumentHttpClientRequestDuration(c.MetricsHttpClientRequestsDurations, httpClient.Transport)
		}
	}
	observers := map[string][]prometheus.ObserverVec{}
	if c.MetricsHttpClientRequestsTraceConnection != nil {
		observers["connection"] = c.MetricsHttpClientRequestsTraceConnection
	}
	if c.MetricsHttpClientRequestsTraceDns != nil {
		observers["dns"] = c.MetricsHttpClientRequestsTraceDns
	}
	if c.MetricsHttpClientRequestsTraceConnect != nil {
		observers["connect"] = c.MetricsHttpClientRequestsTraceConnect
	}
	if c.MetricsHttpClientRequestsTraceTls != nil {
		observers["tls"] = c.MetricsHttpClientRequestsTraceTls
	}
	if c.MetricsHttpClientRequestsTraceRequest != nil {
		observers["request"] = c.MetricsHttpClientRequestsTraceRequest
	}
	if c.MetricsHttpClientRequestLabelFunc != nil {
		httpClient.Transport = metrics.InstrumentHttpClientRequestTrace(observers, httpClient.Transport, c.MetricsHttpClientRequestLabelFunc)
	} else {
		httpClient.Transport = metrics.InstrumentHttpClientRequestTrace(observers, httpClient.Transport)
	}

	if c.MetricsHttpClientRequestsCounter != nil {
		if c.MetricsHttpClientRequestLabelFunc != nil {
			httpClient.Transport = metrics.InstrumentHttpClientRequestCounter(c.MetricsHttpClientRequestsCounter, httpClient.Transport, c.MetricsHttpClientRequestLabelFunc)
		} else {
			httpClient.Transport = metrics.InstrumentHttpClientRequestCounter(c.MetricsHttpClientRequestsCounter, httpClient.Transport)
		}
	}
	if c.MetricsHttpClientRequestsInFlight != nil {
		httpClient.Transport = promhttp.InstrumentRoundTripperInFlight(c.MetricsHttpClientRequestsInFlight, httpClient.Transport)
	}
	return httpClient
}
