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
		MetricsHttpClientRequestLabelFuncs    []metrics.HttpClientRequestLabelFunc
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
	if gauge == nil {
		gauge = defaultMetricsHttpClientRequestsInFlight
	}
	return func(options *ClientOptions) {
		prometheus.Register(gauge)
		options.MetricsHttpClientRequestsInFlight = gauge
	}
}
func ClientMetricsHttpClientRequestsCounter(counter *prometheus.CounterVec) ClientOption {
	if counter == nil {
		counter = defaultMetricsHttpClientRequestsCounter
	}
	return func(options *ClientOptions) {
		prometheus.Register(counter)
		options.MetricsHttpClientRequestsCounter = counter
	}
}
func ClientMetricsHttpClientRequestsDuration(observers []prometheus.ObserverVec) ClientOption {
	if observers == nil {
		observers = []prometheus.ObserverVec{defaultMetricsHttpClientRequests}
	}
	return func(options *ClientOptions) {
		for _, observer := range observers {
			prometheus.Register(observer)
		}
		options.MetricsHttpClientRequestsDurations = observers
	}
}
func ClientMetricsHttpClientRequestsTraceConnection(observers []prometheus.ObserverVec) ClientOption {
	if observers == nil {
		observers = []prometheus.ObserverVec{defaultMetricsHttpClientRequestsTraceConnection}
	}
	return func(options *ClientOptions) {
		for _, observer := range observers {
			prometheus.Register(observer)
		}
		options.MetricsHttpClientRequestsTraceConnection = observers
	}
}
func ClientMetricsHttpClientRequestsTraceDns(observers []prometheus.ObserverVec) ClientOption {
	if observers == nil {
		observers = []prometheus.ObserverVec{defaultMetricsHttpClientRequestsTraceDns}
	}
	return func(options *ClientOptions) {
		for _, observer := range observers {
			prometheus.Register(observer)
		}
		options.MetricsHttpClientRequestsTraceDns = observers
	}
}
func ClientMetricsHttpClientRequestsTraceConnect(observers []prometheus.ObserverVec) ClientOption {
	if observers == nil {
		observers = []prometheus.ObserverVec{defaultMetricsHttpClientRequestsTraceConnect}
	}
	return func(options *ClientOptions) {
		for _, observer := range observers {
			prometheus.Register(observer)
		}
		options.MetricsHttpClientRequestsTraceConnect = observers
	}
}
func ClientMetricsHttpClientRequestsTraceTls(observers []prometheus.ObserverVec) ClientOption {
	if observers == nil {
		observers = []prometheus.ObserverVec{defaultMetricsHttpClientRequestsTraceTls}
	}
	return func(options *ClientOptions) {
		for _, observer := range observers {
			prometheus.Register(observer)
		}
		options.MetricsHttpClientRequestsTraceTls = observers
	}
}
func ClientMetricsHttpClientRequestsTraceRequest(observers []prometheus.ObserverVec) ClientOption {
	if observers == nil {
		observers = []prometheus.ObserverVec{defaultMetricsHttpClientRequestsTraceRequest}
	}
	return func(options *ClientOptions) {
		for _, observer := range observers {
			prometheus.Register(observer)
		}
		options.MetricsHttpClientRequestsTraceRequest = observers
	}
}
func ClientMetricsHttpClientRequestLabelFunc(httpClientRequestLabelFuncs ...metrics.HttpClientRequestLabelFunc) ClientOption {
	return func(options *ClientOptions) {
		options.MetricsHttpClientRequestLabelFuncs = httpClientRequestLabelFuncs
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
		if c.MetricsHttpClientRequestLabelFuncs != nil {
			for _, metricsHttpClientRequestLabelFunc := range c.MetricsHttpClientRequestLabelFuncs {
				httpClient.Transport = metrics.InstrumentHttpClientRequest(c.MetricsHttpClientRequestsDurations, httpClient.Transport, metricsHttpClientRequestLabelFunc)
			}
		} else {
			httpClient.Transport = metrics.InstrumentHttpClientRequest(c.MetricsHttpClientRequestsDurations, httpClient.Transport)
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
	if c.MetricsHttpClientRequestLabelFuncs != nil {
		for _, metricsHttpClientRequestLabelFunc := range c.MetricsHttpClientRequestLabelFuncs {
			httpClient.Transport = metrics.InstrumentHttpClientRequestTrace(observers, httpClient.Transport, metricsHttpClientRequestLabelFunc)
		}
	} else {
		httpClient.Transport = metrics.InstrumentHttpClientRequestTrace(observers, httpClient.Transport)
	}
	if c.MetricsHttpClientRequestsCounter != nil {
		if c.MetricsHttpClientRequestLabelFuncs != nil {
			for _, metricsHttpClientRequestLabelFunc := range c.MetricsHttpClientRequestLabelFuncs {
				httpClient.Transport = metrics.InstrumentHttpClientRequestCounter(c.MetricsHttpClientRequestsCounter, httpClient.Transport, metricsHttpClientRequestLabelFunc)
			}
		} else {
			httpClient.Transport = metrics.InstrumentHttpClientRequestCounter(c.MetricsHttpClientRequestsCounter, httpClient.Transport)
		}
	}
	if c.MetricsHttpClientRequestsInFlight != nil {
		httpClient.Transport = promhttp.InstrumentRoundTripperInFlight(c.MetricsHttpClientRequestsInFlight, httpClient.Transport)
	}
	return httpClient
}
