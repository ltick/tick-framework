package ltick

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ltick/tick-framework/utility"
	"github.com/ltick/tick-routing"
	"github.com/ltick/tick-routing/access"
	"github.com/ltick/tick-routing/fault"
	"github.com/prometheus/client_golang/prometheus"
	"io"
	"os"
)

var (
	errNewDefaultServer = "ltick: new default server"
	errProxyConfig      = "ltick: proxy config '%v'"
)

var (
	// Ltick
	defaultEnvPrefix            = "LTICK"
	defaultConfigFile           = "etc/ltick.json"
	defaultlogWriter  io.Writer = os.Stdout
	// Server
	defaultServerPort                        uint          = 80
	defaultServerLogWriter                   io.Writer     = os.Stdout
	defaultServerGracefulStopTimeoutDuration time.Duration = 120 * time.Second
	// Metrics Http Server
	defaultMetricsHttpServerRequestsDurationHistogram     *prometheus.HistogramVec
	defaultMetricsHttpServerRequestsResponseSizeHistogram *prometheus.HistogramVec
	defaultMetricsHttpServerRequestsRequestSizeHistogram  *prometheus.HistogramVec
	// Metrics Http Client
	defaultMetricsHttpClientRequestsInFlight                 prometheus.Gauge
	defaultMetricsHttpClientRequestsCounter                  *prometheus.CounterVec
	defaultMetricsHttpClientRequestsDurationHistogram        *prometheus.HistogramVec
	defaultMetricsHttpClientRequestsTraceConnectionHistogram *prometheus.HistogramVec
	defaultMetricsHttpClientRequestsTraceConnectHistogram    *prometheus.HistogramVec
	defaultMetricsHttpClientRequestsTraceDnsHistogram        *prometheus.HistogramVec
	defaultMetricsHttpClientRequestsTraceTlsHistogram        *prometheus.HistogramVec
	defaultMetricsHttpClientRequestsTraceRequestHistogram    *prometheus.HistogramVec
)

var defaultEngineCallback Callback

func SetDefaultEngineCallback(c Callback) {
	defaultEngineCallback = c
}

func DefaultConfigFile() string {
	return defaultConfigFile
}

var defaultDotenvFile = ".env"

func DefaultDotenvFile() string {
	return defaultDotenvFile
}

var defaultConfigReloadTime = 120 * time.Second

func DefaultConfigReloadTime() time.Duration {
	return defaultConfigReloadTime
}

var CustomDefaultLogFunc utility.LogFunc

func SetDefaultLogFunc(defaultLogFunc utility.LogFunc) {
	CustomDefaultLogFunc = defaultLogFunc
}

func DefaultLogFunc(ctx context.Context, format string, data ...interface{}) {
	if CustomDefaultLogFunc != nil {
		CustomDefaultLogFunc(ctx, format, data...)
	} else {
		log.Printf(format, data...)
	}
}

var CustomDefaultErrorLogFunc fault.LogFunc

func SetDefaultErrorLogFunc(defaultErrorLogFunc fault.LogFunc) {
	CustomDefaultErrorLogFunc = defaultErrorLogFunc
}

func DefaultErrorLogFunc() fault.LogFunc {
	if CustomDefaultErrorLogFunc != nil {
		return CustomDefaultErrorLogFunc
	} else {
		return log.Printf
	}
}

func DefaultAccessLogFunc(c *routing.Context, rw *access.LogResponseWriter, elapsed float64) {
	//来源请求ID
	forwardRequestId := c.Get("uniqid")
	//请求ID
	requestId := c.Get("requestId")
	//客户端IP
	clientIP := c.Get("clientIP")
	//服务端IP
	serverAddress := c.Get("serverAddress")
	requestLine := fmt.Sprintf("%s %s %s", c.Request.Method, c.Request.RequestURI, c.Request.Proto)
	debug := new(bool)
	if c.Get("DEBUG") != nil {
		*debug = c.Get("DEBUG").(bool)
	}
	if *debug {
		DefaultLogFunc(c.Context, `LTICK_ACCESS|%s|%s|%s|%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "%v" "%v"`, forwardRequestId, requestId, serverAddress, clientIP, c.Request.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, c.Request.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, c.Request.Header.Get("Referer"), c.Request.Header.Get("User-Agent"), c.Request.RemoteAddr, serverAddress, c.Request.Header, rw.Header())
	} else {
		DefaultLogFunc(c.Context, `LTICK_ACCESS|%s|%s|%s|%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "-" "-"`, forwardRequestId, requestId, serverAddress, clientIP, c.Request.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, c.Request.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, c.Request.Header.Get("Referer"), c.Request.Header.Get("User-Agent"), c.Request.RemoteAddr, serverAddress)
	}
	if *debug {
		DefaultLogFunc(c.Context, `%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "%v" "%v"`, clientIP, c.Request.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, c.Request.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, c.Request.Header.Get("Referer"), c.Request.Header.Get("User-Agent"), c.Request.RemoteAddr, serverAddress, c.Request.Header, rw.Header())
	} else {
		DefaultLogFunc(c.Context, `%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "-" "-"`, clientIP, c.Request.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, c.Request.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, c.Request.Header.Get("Referer"), c.Request.Header.Get("User-Agent"), c.Request.RemoteAddr, serverAddress)
	}
}

func init() {
	// Http Server
	defaultMetricsHttpServerRequestsDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_requests_duration_seconds",
		Help:    "A histogram of request latencies for requests.",
		Buckets: prometheus.DefBuckets,
	},
		[]string{"server_addr", "host", "method", "path", "status"},
	)
	prometheus.MustRegister(defaultMetricsHttpServerRequestsDurationHistogram)
	defaultMetricsHttpServerRequestsResponseSizeHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_server_requests_response_size_bytes",
			Help:    "A histogram of response size for requests.",
			Buckets: []float64{200, 500, 900, 1500},
		},
		[]string{"server_addr", "host", "method", "path", "status"},
	)
	prometheus.MustRegister(defaultMetricsHttpServerRequestsResponseSizeHistogram)
	defaultMetricsHttpServerRequestsRequestSizeHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_server_requests_request_size_bytes",
			Help:    "A histogram of request size for requests.",
			Buckets: []float64{200, 500, 900, 1500},
		},
		[]string{"server_addr", "host", "method", "path", "status"},
	)
	prometheus.MustRegister(defaultMetricsHttpServerRequestsRequestSizeHistogram)
	// Http Client
	defaultMetricsHttpClientRequestsInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_client_requests_in_flight",
		Help: "A gauge of in-flight requests for the wrapped client.",
	})
	prometheus.MustRegister(defaultMetricsHttpClientRequestsInFlight)
	defaultMetricsHttpClientRequestsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_client_requests_total",
			Help: "A counter of requests from the wrapped client.",
		},
		[]string{"server_addr", "host", "method", "path", "status"},
	)
	prometheus.MustRegister(defaultMetricsHttpClientRequestsCounter)
	defaultMetricsHttpClientRequestsDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_duration_seconds",
		Help:    "A histogram of request latencies for requests.",
		Buckets: []float64{.25, .5, 1, 2.5, 5, 10},
	},
		[]string{"server_addr", "host", "method", "path", "status"},
	)
	prometheus.MustRegister(defaultMetricsHttpClientRequestsDurationHistogram)
	defaultMetricsHttpClientRequestsTraceConnectionHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_connection_duration_seconds",
		Help:    "A histogram of request latencies for connection.",
		Buckets: []float64{.005, .01, .02, .03},
	},
		[]string{"event", "server_addr", "host", "method", "path"},
	)
	prometheus.MustRegister(defaultMetricsHttpClientRequestsTraceConnectionHistogram)
	defaultMetricsHttpClientRequestsTraceConnectHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_connect_duration_seconds",
		Help:    "A histogram of request latencies for connect.",
		Buckets: []float64{.005, .01, .02, .03},
	},
		[]string{"event", "server_addr", "host", "method", "path"},
	)
	prometheus.MustRegister(defaultMetricsHttpClientRequestsTraceConnectHistogram)
	defaultMetricsHttpClientRequestsTraceDnsHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_dns_duration_seconds",
		Help:    "A histogram of request latencies for dns.",
		Buckets: []float64{.005, .01, .02, .03},
	},
		[]string{"event", "server_addr", "host", "method", "path"},
	)
	prometheus.MustRegister(defaultMetricsHttpClientRequestsTraceDnsHistogram)
	defaultMetricsHttpClientRequestsTraceTlsHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_tls_duration_seconds",
		Help:    "A histogram of request latencies for tls.",
		Buckets: []float64{.005, .01, .02, .03},
	},
		[]string{"event", "server_addr", "host", "method", "path"},
	)
	prometheus.MustRegister(defaultMetricsHttpClientRequestsTraceTlsHistogram)
	defaultMetricsHttpClientRequestsTraceRequestHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_request_duration_seconds",
		Help:    "A histogram of request latencies for request.",
		Buckets: []float64{.005, .01, .02, .03},
	},
		[]string{"event", "server_addr", "host", "method", "path"},
	)
	prometheus.MustRegister(defaultMetricsHttpClientRequestsTraceRequestHistogram)
}
