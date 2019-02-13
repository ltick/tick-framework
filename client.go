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
	DEFAULT_MAX_IDLE_CONNS          = 1000
	DEFAULT_MAX_IDLE_CONNS_PER_HOST = 1000
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
			options.MetricsHttpClientRequestsInFlight = defaultMetricsHttpClientRequestsInFlight
		}

	}
}
func ClientMetricsHttpClientRequestsCounter(counter *prometheus.CounterVec) ClientOption {
	return func(options *ClientOptions) {
		if counter != nil {
			options.MetricsHttpClientRequestsCounter = counter
		} else {
			options.MetricsHttpClientRequestsCounter = defaultMetricsHttpClientRequestsCounter
		}
	}
}
func ClientMetricsHttpClientRequestsDuration(histogram prometheus.ObserverVec) ClientOption {
	if histogram == nil {
		histogram = defaultMetricsHttpClientRequestsDurationSummary
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsDurations = []prometheus.ObserverVec{histogram}
	}
}
func ClientMetricsHttpClientRequestsTraceConnection(histogram prometheus.ObserverVec) ClientOption {
	if histogram == nil {
		histogram = defaultMetricsHttpClientRequestsTraceConnectionSummary
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsTraceConnection = []prometheus.ObserverVec{histogram}
	}
}
func ClientMetricsHttpClientRequestsTraceDns(histogram prometheus.ObserverVec) ClientOption {
	if histogram == nil {
		histogram = defaultMetricsHttpClientRequestsTraceDnsSummary
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsTraceDns = []prometheus.ObserverVec{histogram}
	}
}
func ClientMetricsHttpClientRequestsTraceConnect(histogram prometheus.ObserverVec) ClientOption {
	if histogram == nil {
		histogram = defaultMetricsHttpClientRequestsTraceConnectSummary
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsTraceConnect = []prometheus.ObserverVec{histogram}
	}
}
func ClientMetricsHttpClientRequestsTraceTls(histogram prometheus.ObserverVec) ClientOption {
	if histogram == nil {
		histogram = defaultMetricsHttpClientRequestsTraceTlsSummary
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsTraceTls = []prometheus.ObserverVec{histogram}
	}
}
func ClientMetricsHttpClientRequestsTraceRequest(histogram prometheus.ObserverVec) ClientOption {
	if histogram == nil {
		histogram = defaultMetricsHttpClientRequestsTraceRequestSummary
	}
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestsTraceRequest = []prometheus.ObserverVec{histogram}
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
