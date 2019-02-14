package metrics

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"
	"time"

	"github.com/ltick/tick-framework/utility"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
)

type HttpClientRequestLabelFunc func(obs prometheus.Collector, r *http.Request, rsp *http.Response, customLabels ...prometheus.Labels) prometheus.Labels
type HttpServerRequestLabelFunc func(obs prometheus.Collector, d Delegator, r *http.Request, customLabels ...prometheus.Labels) prometheus.Labels

func defaultMetricsHttpClientRequestLabelFunc(obs prometheus.Collector, r *http.Request, rsp *http.Response, customLabels ...prometheus.Labels) prometheus.Labels {
	serverAddr, host, method, uri, status := checkLabels(obs)
	reqHost := r.Host
	if reqHost == "" {
		reqHost = r.URL.Host
	}
	reqUri := r.URL.Path
	if reqUri == "" {
		reqUri = r.URL.RawPath
	}
	reqServerAddr, _ := utility.GetServerAddress()
	if rsp != nil {
		return labels(serverAddr, host, method, uri, status, reqServerAddr, reqHost, r.Method, reqUri, rsp.StatusCode, customLabels...)
	} else {
		return labels(serverAddr, host, method, uri, status, reqServerAddr, reqHost, r.Method, reqUri, 0, customLabels...)
	}
}

func defaultMetricsHttpServerRequestLabelFunc(obs prometheus.Collector, d Delegator, r *http.Request, customLabels ...prometheus.Labels) prometheus.Labels {
	serverAddr, host, method, uri, status := checkLabels(obs)
	reqHost := r.Host
	if reqHost == "" {
		reqHost = r.URL.Host
	}
	reqUri := r.URL.Path
	if reqUri == "" {
		reqUri = r.URL.RawPath
	}
	reqServerAddr, _ := utility.GetServerAddress()
	return labels(serverAddr, host, method, uri, status, reqServerAddr, reqHost, r.Method, reqUri, d.Status(), customLabels...)
}

// magicString is used for the hacky label test in checkLabels. Remove once fixed.
const magicString = "zZgWfBxLqvG8kc8IMv3POi2Bb0tZI3vAnBx+gBaFi9FyPzB/CzKUer1yufDa"

// InstrumentHttpServerRequestsDuration is a middleware that wraps the provided
// http.Handler to observe the request duration with the provided ObserverVec.
// The ObserverVec must have zero, one, or two non-const non-curried labels. For
// those, the only allowed label names are "status" and "method". The function
// panics otherwise. The Observe method of the Observer in the ObserverVec is
// called with the request duration in seconds. Partitioning happens by HTTP
// status code and/or HTTP method if the respective instance label names are
// present in the ObserverVec. For unpartitioned observations, use an
// ObserverVec with zero labels. Note that partitioning of Histograms is
// expensive and should be used judiciously.
//
// If the wrapped Handler does not set a status code, a status code of 200 is assumed.
//
// If the wrapped Handler panics, no values are reported.
//
// Note that this method is only guaranteed to never observe negative durations
// if used with Go1.9+.
func InstrumentHttpServerRequestsDuration(observers []prometheus.ObserverVec, next http.Handler, serverRequestLabelFuncs ...HttpServerRequestLabelFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		d := newDelegator(w, nil)
		next.ServeHTTP(d, r)
		serverRequestLabelFunc := defaultMetricsHttpServerRequestLabelFunc
		if len(serverRequestLabelFuncs) > 0 {
			serverRequestLabelFunc = serverRequestLabelFuncs[0]
		}
		for _, obs := range observers {
			labels := serverRequestLabelFunc(obs, d, r)
			obs.With(labels).Observe(time.Since(now).Seconds())
		}
	})
}

// InstrumentHttpServerRequestsRequestSize is a middleware that wraps the provided
// http.Handler to observe the request size with the provided ObserverVec.  The
// ObserverVec must have zero, one, or two non-const non-curried labels. For
// those, the only allowed label names are "status" and "method". The function
// panics otherwise. The Observe method of the Observer in the ObserverVec is
// called with the request size in bytes. Partitioning happens by HTTP status
// status and/or HTTP method if the respective instance label names are present in
// the ObserverVec. For unpartitioned observations, use an ObserverVec with zero
// labels. Note that partitioning of Histograms is expensive and should be used
// judiciously.
//
// If the wrapped Handler does not set a status code, a status code of 200 is assumed.
//
// If the wrapped Handler panics, no values are reported.
//
// See the example for InstrumentHttpServerRequestsDuration for example usage.
func InstrumentHttpServerRequestsRequestSize(observers []prometheus.ObserverVec, next http.Handler, serverRequestLabelFuncs ...HttpServerRequestLabelFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d := newDelegator(w, nil)
		next.ServeHTTP(d, r)
		size := computeApproximateRequestSize(r)
		serverRequestLabelFunc := defaultMetricsHttpServerRequestLabelFunc
		if len(serverRequestLabelFuncs) > 0 {
			serverRequestLabelFunc = serverRequestLabelFuncs[0]
		}
		for _, obs := range observers {
			labels := serverRequestLabelFunc(obs, d, r)
			obs.With(labels).Observe(float64(size))
		}
	})
}

// InstrumentHttpServerRequestsResponseSize is a middleware that wraps the provided
// http.Handler to observe the response size with the provided ObserverVec.  The
// ObserverVec must have zero, one, or two non-const non-curried labels. For
// those, the only allowed label names are "status" and "method". The function
// panics otherwise. The Observe method of the Observer in the ObserverVec is
// called with the response size in bytes. Partitioning happens by HTTP status
// status and/or HTTP method if the respective instance label names are present in
// the ObserverVec. For unpartitioned observations, use an ObserverVec with zero
// labels. Note that partitioning of Histograms is expensive and should be used
// judiciously.
//
// If the wrapped Handler does not set a status code, a status code of 200 is assumed.
//
// If the wrapped Handler panics, no values are reported.
//
// See the example for InstrumentHttpServerRequestsDuration for example usage.
func InstrumentHttpServerRequestsResponseSize(observers []prometheus.ObserverVec, next http.Handler, serverRequestLabelFuncs ...HttpServerRequestLabelFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d := newDelegator(w, nil)
		next.ServeHTTP(d, r)
		serverRequestLabelFunc := defaultMetricsHttpServerRequestLabelFunc
		if len(serverRequestLabelFuncs) > 0 {
			serverRequestLabelFunc = serverRequestLabelFuncs[0]
		}
		for _, obs := range observers {
			labels := serverRequestLabelFunc(obs, d, r)
			obs.With(labels).Observe(float64(d.Written()))
		}
	})
}

// InstrumentHttpClientRequestCounter is a middleware that wraps the provided
// http.RoundTripper to observe the request result with the provided CounterVec.
// The CounterVec must have zero, one, or two non-const non-curried labels. For
// those, the only allowed label names are "code" and "method". The function
// panics otherwise. Partitioning of the CounterVec happens by HTTP status code
// and/or HTTP method if the respective instance label names are present in the
// CounterVec. For unpartitioned counting, use a CounterVec with zero labels.
//
// If the wrapped RoundTripper panics or returns a non-nil error, the Counter
// is not incremented.
//
// See the example for ExampleInstrumentHttpClientRequestDuration for example usage.
func InstrumentHttpClientRequestCounter(counter *prometheus.CounterVec, next http.RoundTripper, clientRequestLabelFuncs ...HttpClientRequestLabelFunc) promhttp.RoundTripperFunc {
	return promhttp.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		resp, err := next.RoundTrip(r)
		if err == nil {
			clientRequestLabelFunc := defaultMetricsHttpClientRequestLabelFunc
			if len(clientRequestLabelFuncs) > 0 {
				clientRequestLabelFunc = clientRequestLabelFuncs[0]
			}
			labels := clientRequestLabelFunc(counter, r, resp)
			counter.With(labels).Inc()
		}
		return resp, err
	})
}

// InstrumentHttpClientRequestDuration is a middleware that wraps the provided
// http.RoundTripper to observe the request duration with the provided
// ObserverVec.  The ObserverVec must have zero, one, or two non-const
// non-curried labels. For those, the only allowed label names are "code" and
// "method". The function panics otherwise. The Observe method of the Observer
// in the ObserverVec is called with the request duration in
// seconds. Partitioning happens by HTTP status code and/or HTTP method if the
// respective instance label names are present in the ObserverVec. For
// unpartitioned observations, use an ObserverVec with zero labels. Note that
// partitioning of Histograms is expensive and should be used judiciously.
//
// If the wrapped RoundTripper panics or returns a non-nil error, no values are
// reported.
//
// Note that this method is only guaranteed to never observe negative durations
// if used with Go1.9+.
func InstrumentHttpClientRequestDuration(observers []prometheus.ObserverVec, next http.RoundTripper, clientRequestLabelFuncs ...HttpClientRequestLabelFunc) promhttp.RoundTripperFunc {
	return promhttp.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		start := time.Now()
		resp, err := next.RoundTrip(r)
		if err == nil {
			clientRequestLabelFunc := defaultMetricsHttpClientRequestLabelFunc
			if len(clientRequestLabelFuncs) > 0 {
				clientRequestLabelFunc = clientRequestLabelFuncs[0]
			}
			for _, obs := range observers {
				labels := clientRequestLabelFunc(obs, r, resp)
				obs.With(labels).Observe(time.Since(start).Seconds())
			}
		}
		return resp, err
	})
}

// InstrumentTrace is used to offer flexibility in instrumenting the available
// httptrace.ClientTrace hook functions. Each function is passed a float64
// representing the time in seconds since the start of the http request. A user
// may choose to use separately buckets Histograms, or implement custom
// instance labels on a per function basis.
type InstrumentTrace struct {
	GotConn              func(float64, *http.Request)
	PutIdleConn          func(float64, *http.Request)
	GotFirstResponseByte func(float64, *http.Request)
	Got100Continue       func(float64, *http.Request)
	DNSStart             func(float64, *http.Request)
	DNSDone              func(float64, *http.Request)
	ConnectStart         func(float64, *http.Request)
	ConnectDone          func(float64, *http.Request)
	TLSHandshakeStart    func(float64, *http.Request)
	TLSHandshakeDone     func(float64, *http.Request)
	WroteHeaders         func(float64, *http.Request)
	Wait100Continue      func(float64, *http.Request)
	WroteRequest         func(float64, *http.Request)
}

// InstrumentHttpClientRequestTrace is a middleware that wraps the provided
// RoundTripper and reports times to hook functions provided in the
// InstrumentTrace struct. Hook functions that are not present in the provided
// InstrumentTrace struct are ignored. Times reported to the hook functions are
// time since the start of the request. Only with Go1.9+, those times are
// guaranteed to never be negative. (Earlier Go versions are not using a
// monotonic clock.) Note that partitioning of Histograms is expensive and
// should be used judiciously.
//
// For hook functions that receive an error as an argument, no observations are
// made in the event of a non-nil error value.
//
// See the example for ExampleInstrumentRoundTripperDuration for example usage.
func InstrumentHttpClientRequestTrace(observers map[string][]prometheus.ObserverVec, next http.RoundTripper, clientRequestLabelFuncs ...HttpClientRequestLabelFunc) promhttp.RoundTripperFunc {
	return promhttp.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		clientRequestLabelFunc := defaultMetricsHttpClientRequestLabelFunc
		if len(clientRequestLabelFuncs) > 0 {
			clientRequestLabelFunc = clientRequestLabelFuncs[0]
		}
		// Define functions for the available httptrace.ClientTrace hook
		// functions that we want to instrument.
		it := &InstrumentTrace{}
		if _, ok := observers["connection"]; ok {
			it.GotConn = func(t float64, r *http.Request) {
				for _, obs := range observers["connection"] {
					labels := clientRequestLabelFunc(obs, r, nil, prometheus.Labels{"event": "got_conn"})
					obs.With(labels).Observe(t)
				}
			}
			it.PutIdleConn = func(t float64, r *http.Request) {
				for _, obs := range observers["connection"] {
					labels := clientRequestLabelFunc(obs, r, nil, prometheus.Labels{"event": "put_idle_conn"})
					obs.With(labels).Observe(t)
				}
			}
		}
		if _, ok := observers["dns"]; ok {
			it.DNSStart = func(t float64, r *http.Request) {
				for _, obs := range observers["dns"] {
					labels := clientRequestLabelFunc(obs, r, nil, prometheus.Labels{"event": "dns_start"})
					obs.With(labels).Observe(t)
				}
			}
			it.DNSDone = func(t float64, r *http.Request) {
				for _, obs := range observers["dns"] {
					labels := clientRequestLabelFunc(obs, r, nil, prometheus.Labels{"event": "dns_done"})
					obs.With(labels).Observe(t)
				}
			}
		}
		if _, ok := observers["connect"]; ok {
			it.ConnectStart = func(t float64, r *http.Request) {
				for _, obs := range observers["connect"] {
					labels := clientRequestLabelFunc(obs, r, nil, prometheus.Labels{"event": "connect_start"})
					obs.With(labels).Observe(t)
				}
			}
			it.ConnectDone = func(t float64, r *http.Request) {
				for _, obs := range observers["connect"] {
					labels := clientRequestLabelFunc(obs, r, nil, prometheus.Labels{"event": "connect_done"})
					obs.With(labels).Observe(t)
				}
			}
		}
		if _, ok := observers["tls"]; ok {
			it.TLSHandshakeStart = func(t float64, r *http.Request) {
				for _, obs := range observers["tls"] {
					labels := clientRequestLabelFunc(obs, r, nil, prometheus.Labels{"event": "tls_handshake_start"})
					obs.With(labels).Observe(t)
				}
			}
			it.TLSHandshakeDone = func(t float64, r *http.Request) {
				for _, obs := range observers["tls"] {
					labels := clientRequestLabelFunc(obs, r, nil, prometheus.Labels{"event": "tls_handshake_done"})
					obs.With(labels).Observe(t)
				}
			}
		}
		if _, ok := observers["request"]; ok {
			it.WroteHeaders = func(t float64, r *http.Request) {
				for _, obs := range observers["request"] {
					labels := clientRequestLabelFunc(obs, r, nil, prometheus.Labels{"event": "wrote_headers"})
					obs.With(labels).Observe(t)
				}
			}
			it.WroteRequest = func(t float64, r *http.Request) {
				for _, obs := range observers["request"] {
					labels := clientRequestLabelFunc(obs, r, nil, prometheus.Labels{"event": "wrote_request"})
					obs.With(labels).Observe(t)
				}
			}
			it.GotFirstResponseByte = func(t float64, r *http.Request) {
				for _, obs := range observers["request"] {
					labels := clientRequestLabelFunc(obs, r, nil, prometheus.Labels{"event": "got_first_response_byte"})
					obs.With(labels).Observe(t)
				}
			}
			it.Got100Continue = func(t float64, r *http.Request) {
				for _, obs := range observers["request"] {
					labels := clientRequestLabelFunc(obs, r, nil, prometheus.Labels{"event": "got_100_continue"})
					obs.With(labels).Observe(t)
				}
			}
			it.Wait100Continue = func(t float64, r *http.Request) {
				for _, obs := range observers["request"] {
					labels := clientRequestLabelFunc(obs, r, nil, prometheus.Labels{"event": "wait_100_continue"})
					obs.With(labels).Observe(t)
				}
			}
		}
		start := time.Now()

		trace := &httptrace.ClientTrace{
			GotConn: func(_ httptrace.GotConnInfo) {
				if it.GotConn != nil {
					it.GotConn(time.Since(start).Seconds(), r)
				}
			},
			PutIdleConn: func(err error) {
				if err != nil {
					return
				}
				if it.PutIdleConn != nil {
					it.PutIdleConn(time.Since(start).Seconds(), r)
				}
			},
			DNSStart: func(_ httptrace.DNSStartInfo) {
				if it.DNSStart != nil {
					it.DNSStart(time.Since(start).Seconds(), r)
				}
			},
			DNSDone: func(_ httptrace.DNSDoneInfo) {
				if it.DNSDone != nil {
					it.DNSDone(time.Since(start).Seconds(), r)
				}
			},
			ConnectStart: func(_, _ string) {
				if it.ConnectStart != nil {
					it.ConnectStart(time.Since(start).Seconds(), r)
				}
			},
			ConnectDone: func(_, _ string, err error) {
				if err != nil {
					return
				}
				if it.ConnectDone != nil {
					it.ConnectDone(time.Since(start).Seconds(), r)
				}
			},
			GotFirstResponseByte: func() {
				if it.GotFirstResponseByte != nil {
					it.GotFirstResponseByte(time.Since(start).Seconds(), r)
				}
			},
			Got100Continue: func() {
				if it.Got100Continue != nil {
					it.Got100Continue(time.Since(start).Seconds(), r)
				}
			},
			TLSHandshakeStart: func() {
				if it.TLSHandshakeStart != nil {
					it.TLSHandshakeStart(time.Since(start).Seconds(), r)
				}
			},
			TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
				if err != nil {
					return
				}
				if it.TLSHandshakeDone != nil {
					it.TLSHandshakeDone(time.Since(start).Seconds(), r)
				}
			},
			WroteHeaders: func() {
				if it.WroteHeaders != nil {
					it.WroteHeaders(time.Since(start).Seconds(), r)
				}
			},
			Wait100Continue: func() {
				if it.Wait100Continue != nil {
					it.Wait100Continue(time.Since(start).Seconds(), r)
				}
			},
			WroteRequest: func(_ httptrace.WroteRequestInfo) {
				if it.WroteRequest != nil {
					it.WroteRequest(time.Since(start).Seconds(), r)
				}
			},
		}
		r = r.WithContext(httptrace.WithClientTrace(context.Background(), trace))

		return next.RoundTrip(r)
	})
}

// emptyLabels is a one-time allocation for non-partitioned metrics to avoid
// unnecessary allocations on each request.
var emptyLabels = prometheus.Labels{}

func labels(serverAddr, host, method, uri, status bool, reqServerAddr string, reqHost string, reqMethod string, reqUri string, repStatus int, customLabels ...prometheus.Labels) prometheus.Labels {
	labels := prometheus.Labels{}
	if len(customLabels) > 0 {
		labels = customLabels[0]
	}
	if serverAddr {
		labels["server_addr"] = reqServerAddr
	}
	if host {
		labels["host"] = reqHost
	}
	if status {
		labels["status"] = sanitizeStatus(repStatus)
	}
	if method {
		labels["method"] = sanitizeMethod(reqMethod)
	}
	if uri {
		labels["uri"] = reqUri
	}
	return labels
}

func computeApproximateRequestSize(r *http.Request) int {
	s := 0
	if r.URL != nil {
		s += len(r.URL.String())
	}

	s += len(r.Method)
	s += len(r.Proto)
	for name, values := range r.Header {
		s += len(name)
		for _, value := range values {
			s += len(value)
		}
	}
	s += len(r.Host)

	// N.B. r.Form and r.MultipartForm are assumed to be included in r.URL.

	if r.ContentLength != -1 {
		s += int(r.ContentLength)
	}
	return s
}

func sanitizeMethod(m string) string {
	switch m {
	case "GET", "get":
		return "GET"
	case "PUT", "put":
		return "PUT"
	case "HEAD", "head":
		return "HEAD"
	case "POST", "post":
		return "POST"
	case "DELETE", "delete":
		return "DELETE"
	case "CONNECT", "connect":
		return "CONNECT"
	case "OPTIONS", "options":
		return "OPTIONS"
	case "NOTIFY", "notify":
		return "NOTIFY"
	default:
		return strings.ToUpper(m)
	}
}

// If the wrapped http.Handler has not set a status code, i.e. the value is
// currently 0, santizeCode will return 200, for consistency with behavior in
// the stdlib.
func sanitizeStatus(s int) string {
	switch s {
	case 100:
		return "100"
	case 101:
		return "101"

	case 200, 0:
		return "200"
	case 201:
		return "201"
	case 202:
		return "202"
	case 203:
		return "203"
	case 204:
		return "204"
	case 205:
		return "205"
	case 206:
		return "206"

	case 300:
		return "300"
	case 301:
		return "301"
	case 302:
		return "302"
	case 304:
		return "304"
	case 305:
		return "305"
	case 307:
		return "307"

	case 400:
		return "400"
	case 401:
		return "401"
	case 402:
		return "402"
	case 403:
		return "403"
	case 404:
		return "404"
	case 405:
		return "405"
	case 406:
		return "406"
	case 407:
		return "407"
	case 408:
		return "408"
	case 409:
		return "409"
	case 410:
		return "410"
	case 411:
		return "411"
	case 412:
		return "412"
	case 413:
		return "413"
	case 414:
		return "414"
	case 415:
		return "415"
	case 416:
		return "416"
	case 417:
		return "417"
	case 418:
		return "418"

	case 500:
		return "500"
	case 501:
		return "501"
	case 502:
		return "502"
	case 503:
		return "503"
	case 504:
		return "504"
	case 505:
		return "505"

	case 428:
		return "428"
	case 429:
		return "429"
	case 431:
		return "431"
	case 511:
		return "511"

	default:
		return strconv.Itoa(s)
	}
}

func checkLabels(c prometheus.Collector) (serverAddr, host, method, uri, status bool) {
	// TODO(beorn7): Remove this hacky way to check for instance labels
	// once Descriptors can have their dimensionality queried.
	var (
		desc *prometheus.Desc
		m    prometheus.Metric
		pm   dto.Metric
		lvs  []string
	)

	// Get the Desc from the Collector.
	descc := make(chan *prometheus.Desc, 1)
	c.Describe(descc)

	select {
	case desc = <-descc:
	default:
		panic("no description provided by collector")
	}
	select {
	case <-descc:
		panic("more than one description provided by collector")
	default:
	}

	close(descc)

	// Create a ConstMetric with the Desc. Since we don't know how many
	// variable labels there are, try for as long as it needs.
	for err := errors.New("dummy"); err != nil; lvs = append(lvs, magicString) {
		m, err = prometheus.NewConstMetric(desc, prometheus.UntypedValue, 0, lvs...)
	}

	// Write out the metric into a proto message and look at the labels.
	// If the value is not the magicString, it is a constLabel, which doesn't interest us.
	// If the label is curried, it doesn't interest us.
	// In all other cases, only "status" or "method" is allowed.
	if err := m.Write(&pm); err != nil {
		panic("error checking metric for labels")
	}
	for _, label := range pm.Label {
		name, value := label.GetName(), label.GetValue()
		if value != magicString || isLabelCurried(c, name) {
			continue
		}
		switch name {
		case "server_addr":
			serverAddr = true
		case "host":
			host = true
		case "method":
			method = true
		case "uri":
			uri = true
		case "status":
			status = true
		case "event":
		default:
			panic("metric partitioned with non-supported labels")
		}
	}
	return
}

func isLabelCurried(c prometheus.Collector, label string) bool {
	// This is even hackier than the label test above.
	// We essentially try to curry again and see if it works.
	// But for that, we need to type-convert to the two
	// types we use here, ObserverVec or *CounterVec.
	switch v := c.(type) {
	case *prometheus.CounterVec:
		if _, err := v.CurryWith(prometheus.Labels{label: "dummy"}); err == nil {
			return false
		}
	case prometheus.ObserverVec:
		if _, err := v.CurryWith(prometheus.Labels{label: "dummy"}); err == nil {
			return false
		}
	default:
		panic("unsupported metric vec type")
	}
	return true
}
