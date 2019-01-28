package metrics

import (
	"context"

	"github.com/juju/errors"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	errSummaryHasRegistered   = "metrics: summary '%s' has registered"
	errRegisterSummary        = "metrics: register summary '%s'"
	errCounterHasRegistered   = "metrics: counter '%s' has registered"
	errRegisterCounter        = "metrics: register counter '%s'"
	errGaugeHasRegistered     = "metrics: gauge '%s' has registered"
	errRegisterGauge          = "metrics: register gauge '%s'"
	errHistogramHasRegistered = "metrics: histogram '%s' has registered"
	errRegisterHistogram      = "metrics: register histogram '%s'"
)

var m *Metrics

var summarys map[string]*prometheus.SummaryVec
var counters map[string]*prometheus.CounterVec
var gauges map[string]*prometheus.GaugeVec
var histograms map[string]*prometheus.HistogramVec

func init() {
	m = New()
}

// New returns an initialized Viper instance.
func New() *Metrics {
	summarys = make(map[string]*prometheus.SummaryVec, 0)
	counters = make(map[string]*prometheus.CounterVec, 0)
	gauges = make(map[string]*prometheus.GaugeVec, 0)
	histograms = make(map[string]*prometheus.HistogramVec, 0)
	return new(Metrics)
}

type Metrics struct {
}

func (i *Metrics) Prepare(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (i *Metrics) Initiate(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (i *Metrics) OnStartup(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (i *Metrics) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func RegisterSummary(name string, cs *prometheus.SummaryVec) error {
	return m.RegisterSummary(name, cs)
}
func (i *Metrics) RegisterSummary(name string, cs *prometheus.SummaryVec) error {
	if _, ok := summarys[name]; !ok {
		summarys[name] = cs
		prometheus.MustRegister(cs)
	} else {
		return errors.Annotate(errors.Errorf(errSummaryHasRegistered, name), errRegisterSummary)
	}
	return nil
}

func GetSummary(name string) *prometheus.SummaryVec { return m.GetSummary(name) }
func (i *Metrics) GetSummary(name string) (cs *prometheus.SummaryVec) {
	if _, ok := summarys[name]; ok {
		return summarys[name]
	} else {
		return nil
	}
}

func RegisterCounter(name string, cs *prometheus.CounterVec) error {
	return m.RegisterCounter(name, cs)
}
func (i *Metrics) RegisterCounter(name string, cs *prometheus.CounterVec) error {
	if _, ok := counters[name]; !ok {
		counters[name] = cs
		prometheus.MustRegister(cs)
	} else {
		return errors.Annotate(errors.Errorf(errCounterHasRegistered, name), errRegisterCounter)
	}
	return nil
}

func GetCounter(name string) *prometheus.CounterVec { return m.GetCounter(name) }
func (i *Metrics) GetCounter(name string) (cs *prometheus.CounterVec) {
	if _, ok := counters[name]; ok {
		return counters[name]
	} else {
		return nil
	}
}

func RegisterGauge(name string, cs *prometheus.GaugeVec) error {
	return m.RegisterGauge(name, cs)
}
func (i *Metrics) RegisterGauge(name string, cs *prometheus.GaugeVec) error {
	if _, ok := gauges[name]; !ok {
		gauges[name] = cs
		prometheus.MustRegister(cs)
	} else {
		return errors.Annotate(errors.Errorf(errGaugeHasRegistered, name), errRegisterGauge)
	}
	return nil
}

func GetGauge(name string) *prometheus.GaugeVec { return m.GetGauge(name) }
func (i *Metrics) GetGauge(name string) (cs *prometheus.GaugeVec) {
	if _, ok := gauges[name]; ok {
		return gauges[name]
	} else {
		return nil
	}
}

func RegisterHistogram(name string, cs *prometheus.HistogramVec) error {
	return m.RegisterHistogram(name, cs)
}
func (i *Metrics) RegisterHistogram(name string, cs *prometheus.HistogramVec) error {
	if _, ok := histograms[name]; !ok {
		histograms[name] = cs
		prometheus.MustRegister(cs)
	} else {
		return errors.Annotate(errors.Errorf(errHistogramHasRegistered, name), errRegisterHistogram)
	}
	return nil
}

func GetHistogram(name string) *prometheus.HistogramVec { return m.GetHistogram(name) }
func (i *Metrics) GetHistogram(name string) (cs *prometheus.HistogramVec) {
	if _, ok := histograms[name]; ok {
		return histograms[name]
	} else {
		return nil
	}
}
