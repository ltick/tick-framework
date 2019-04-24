package ltick

import (
	"context"
	"fmt"
	"log"
	"time"
	"net/http"

	"github.com/ltick/tick-framework/utility"
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
	defaultServerReadTimeoutDuration         time.Duration = 60 * time.Second
	defaultServerReadHeaderTimeoutDuration   time.Duration = 60 * time.Second
	defaultServerWriteTimeoutDuration        time.Duration = 60 * time.Second
	defaultServerIdleTimeoutDuration         time.Duration = 60 * time.Second
	// Metrics Http Server
	defaultMetricsHttpServerRequestsCounter      *prometheus.CounterVec
	defaultMetricsHttpServerRequests             *prometheus.HistogramVec
	defaultMetricsHttpServerRequestsResponseSize *prometheus.HistogramVec
	defaultMetricsHttpServerRequestsRequestSize  *prometheus.HistogramVec
	defaultMetricsHttpServerRequestsTrace        *prometheus.HistogramVec
	// Metrics Http Client
	defaultMetricsHttpClientRequestsInFlight        prometheus.Gauge
	defaultMetricsHttpClientRequestsCounter         *prometheus.CounterVec
	defaultMetricsHttpClientRequests                *prometheus.HistogramVec
	defaultMetricsHttpClientRequestsTraceConnection *prometheus.HistogramVec
	defaultMetricsHttpClientRequestsTraceConnect    *prometheus.HistogramVec
	defaultMetricsHttpClientRequestsTraceDns        *prometheus.HistogramVec
	defaultMetricsHttpClientRequestsTraceTls        *prometheus.HistogramVec
	defaultMetricsHttpClientRequestsTraceRequest    *prometheus.HistogramVec
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

func DefaultAccessLogFunc(req *http.Request, rw *access.LogResponseWriter, elapsed float64) {
	//来源请求ID
	forwardRequestId := req.Context().Value("uniqid")
	//请求ID
	requestId := req.Context().Value("requestId")
	//客户端IP
	clientIP := req.Context().Value(req.RemoteAddr)
	//服务端IP
	serverAddress := req.Context().Value(http.LocalAddrContextKey)
	requestLine := fmt.Sprintf("%s %s %s", req.Method, req.RequestURI, req.Proto)
	debug := new(bool)
	if req.Context().Value("DEBUG") != nil {
		*debug = req.Context().Value("DEBUG").(bool)
	}
	if *debug {
		DefaultLogFunc(req.Context(), `LTICK_ACCESS|%s|%s|%s|%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "%v" "%v"`, forwardRequestId, requestId, serverAddress, clientIP, req.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, req.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, req.Header.Get("Referer"), req.Header.Get("User-Agent"), req.RemoteAddr, serverAddress, req.Header, rw.Header())
	} else {
		DefaultLogFunc(req.Context(), `LTICK_ACCESS|%s|%s|%s|%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "-" "-"`, forwardRequestId, requestId, serverAddress, clientIP, req.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, req.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, req.Header.Get("Referer"), req.Header.Get("User-Agent"), req.RemoteAddr, serverAddress)
	}
	if *debug {
		DefaultLogFunc(req.Context(), `%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "%v" "%v"`, clientIP, req.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, req.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, req.Header.Get("Referer"), req.Header.Get("User-Agent"), req.RemoteAddr, serverAddress, req.Header, rw.Header())
	} else {
		DefaultLogFunc(req.Context(), `%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "-" "-"`, clientIP, req.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, req.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, req.Header.Get("Referer"), req.Header.Get("User-Agent"), req.RemoteAddr, serverAddress)
	}
}

func init() {
	// Http Server Histogram
	defaultMetricsHttpServerRequests = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_requests_seconds",
		Help:    "A histogram of request latencies for requests.",
		Buckets: []float64{.25, .5, 1, 2.5, 5, 10},
	},
		[]string{"server_addr", "host", "method", "uri", "status"},
	)
	prometheus.MustRegister(defaultMetricsHttpServerRequests)
	defaultMetricsHttpServerRequestsResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_server_requests_response_size_bytes",
			Help:    "A histogram of response size for requests.",
			Buckets: []float64{200, 500, 900, 1500},
		},
		[]string{"server_addr", "host", "method", "uri", "status"},
	)
	prometheus.MustRegister(defaultMetricsHttpServerRequestsResponseSize)
	defaultMetricsHttpServerRequestsRequestSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_server_requests_request_size_bytes",
			Help:    "A histogram of request size for requests.",
			Buckets: []float64{200, 500, 900, 1500},
		},
		[]string{"server_addr", "host", "method", "uri", "status"},
	)
	prometheus.MustRegister(defaultMetricsHttpServerRequestsRequestSize)
	defaultMetricsHttpServerRequestsTrace = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_requests_trace_seconds",
		Help:    "A histogram of request trace latencies for requests.",
		Buckets: []float64{.25, .5, 1, 2.5, 5, 10},
	},
		[]string{"event", "server_addr", "host", "method", "uri", "status"},
	)
	prometheus.MustRegister(defaultMetricsHttpServerRequestsTrace)
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
		[]string{"server_addr", "host", "method", "uri", "status"},
	)
	prometheus.MustRegister(defaultMetricsHttpClientRequestsCounter)
	// Http Client Histogram
	defaultMetricsHttpClientRequests = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_seconds",
		Help:    "A histogram of request latencies for requests.",
		Buckets: []float64{.25, .5, 1, 2.5, 5, 10},
	},
		[]string{"server_addr", "host", "method", "uri", "status"},
	)
	prometheus.MustRegister(defaultMetricsHttpClientRequests)
	defaultMetricsHttpClientRequestsTraceConnection = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_trace_connection_seconds",
		Help:    "A histogram of request trace latencies for connection.",
		Buckets: []float64{.005, .01, .02, .05},
	},
		[]string{"event", "server_addr", "host", "method", "uri"},
	)
	prometheus.MustRegister(defaultMetricsHttpClientRequestsTraceConnection)
	defaultMetricsHttpClientRequestsTraceConnect = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_trace_connect_seconds",
		Help:    "A histogram of request trace latencies for connect.",
		Buckets: []float64{.005, .01, .02, .05},
	},
		[]string{"event", "server_addr", "host", "method", "uri"},
	)
	prometheus.MustRegister(defaultMetricsHttpClientRequestsTraceConnect)
	defaultMetricsHttpClientRequestsTraceDns = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_trace_dns_seconds",
		Help:    "A histogram of request trace latencies for dns.",
		Buckets: []float64{.005, .01, .02, .05},
	},
		[]string{"event", "server_addr", "host", "method", "uri"},
	)
	prometheus.MustRegister(defaultMetricsHttpClientRequestsTraceDns)
	defaultMetricsHttpClientRequestsTraceTls = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_trace_tls_seconds",
		Help:    "A histogram of request trace latencies for tls.",
		Buckets: []float64{.005, .01, .02, .05},
	},
		[]string{"event", "server_addr", "host", "method", "uri"},
	)
	prometheus.MustRegister(defaultMetricsHttpClientRequestsTraceTls)
	defaultMetricsHttpClientRequestsTraceRequest = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_requests_trace_request_seconds",
		Help:    "A histogram of request trace latencies for request.",
		Buckets: []float64{.005, .01, .02, .05},
	},
		[]string{"event", "server_addr", "host", "method", "uri"},
	)
	prometheus.MustRegister(defaultMetricsHttpClientRequestsTraceRequest)
}
