package metrics

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// magicString is used for the hacky label test in checkLabels. Remove once fixed.
const magicString = "zZgWfBxLqvG8kc8IMv3POi2Bb0tZI3vAnBx+gBaFi9FyPzB/CzKUer1yufDa"

// InstrumentHandlerDuration is a middleware that wraps the provided
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
func InstrumentHandlerDuration(obs prometheus.ObserverVec, next http.Handler) http.HandlerFunc {
	scheme, host, method, path, status := checkLabels(obs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		d := newDelegator(w, nil)
		next.ServeHTTP(d, r)
		reqHost := r.Host
		if reqHost == "" {
			reqHost = r.URL.Host
		}
		reqPath := r.URL.Path
		if reqPath == "" {
			reqPath = r.URL.RawPath
		}
		obs.With(labels(scheme, host, method, path, status, r.URL.Scheme, reqHost, r.Method, reqPath, d.Status())).Observe(time.Since(now).Seconds())
	})
}

// InstrumentHandlerRequestSize is a middleware that wraps the provided
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
// See the example for InstrumentHandlerDuration for example usage.
func InstrumentHandlerRequestSize(obs prometheus.ObserverVec, next http.Handler) http.HandlerFunc {
	scheme, host, method, path, status := checkLabels(obs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d := newDelegator(w, nil)
		next.ServeHTTP(d, r)
		size := computeApproximateRequestSize(r)
		reqHost := r.Host
		if reqHost == "" {
			reqHost = r.URL.Host
		}
		reqPath := r.URL.Path
		if reqPath == "" {
			reqPath = r.URL.RawPath
		}
		obs.With(labels(scheme, host, method, path, status, r.URL.Scheme, reqHost, r.Method, reqPath, d.Status())).Observe(float64(size))
	})
}

// InstrumentHandlerResponseSize is a middleware that wraps the provided
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
// See the example for InstrumentHandlerDuration for example usage.
func InstrumentHandlerResponseSize(obs prometheus.ObserverVec, next http.Handler) http.Handler {
	scheme, host, method, path, status := checkLabels(obs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d := newDelegator(w, nil)
		next.ServeHTTP(d, r)
		reqHost := r.Host
		if reqHost == "" {
			reqHost = r.URL.Host
		}
		reqPath := r.URL.Path
		if reqPath == "" {
			reqPath = r.URL.RawPath
		}
		obs.With(labels(scheme, host, method, path, status, r.URL.Scheme, reqHost, r.Method, reqPath, d.Status())).Observe(float64(d.Written()))
	})
}

// emptyLabels is a one-time allocation for non-partitioned metrics to avoid
// unnecessary allocations on each request.
var emptyLabels = prometheus.Labels{}

func labels(scheme, host, method, path, status bool, reqScheme string, reqHost string, reqMethod string, reqPath string, repStatus int) prometheus.Labels {
	labels := prometheus.Labels{}
	if scheme {
		labels["scheme"] = reqScheme
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
	if path {
		labels["path"] = reqPath
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

func checkLabels(c prometheus.Collector) (scheme, host, method, path, status bool) {
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
		case "scheme":
			scheme = true
		case "host":
			host = true
		case "method":
			method = true
		case "path":
			path = true
		case "status":
			status = true
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
