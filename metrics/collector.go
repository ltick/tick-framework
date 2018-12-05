package metrics

import (
	"context"

	"github.com/juju/errors"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	errCollectorHasRegistered  = "metrics: collector '%s' has registered"
	errRegisterCollector       = "metrics: register collector '%s'"
	errRegisterCustomCollector = "metrics: register custom collector '%s'"
)

type Metrics struct {
	Collectors map[string]prometheus.Collector
}

func (i *Metrics) Prepare(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (i *Metrics) Initiate(ctx context.Context) (context.Context, error) {
	i.Collectors = make(map[string]prometheus.Collector, 0)
	return ctx, nil
}
func (i *Metrics) OnStartup(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (i *Metrics) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func (i *Metrics) RegisterCollector(name string, cs prometheus.Collector) error {
	if _, ok := i.Collectors[name]; !ok {
		i.Collectors[name] = cs
		prometheus.MustRegister(cs)
	} else {
		return errors.Annotate(errors.Errorf(errCollectorHasRegistered, name), errRegisterCollector)
	}
	return nil
}
func (i *Metrics) GetCollector(name string) (cs prometheus.Collector) {
	if _, ok := i.Collectors[name]; ok {
		return i.Collectors[name]
	} else {
		return nil
	}
}

func (i *Metrics) RegisterCustomCollector(name string, reg prometheus.Registerer, descs []*prometheus.Desc, zone string, hosts []string) error {
	if _, ok := i.Collectors[name]; !ok {
		c := &ClusterCollector{
			descs: descs,
			hosts: hosts,
		}
		prometheus.WrapRegistererWith(prometheus.Labels{"zone": zone}, reg).MustRegister(c)
		reg.MustRegister(
			prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
			prometheus.NewGoCollector(),
		)
		i.Collectors[name] = c
	} else {
		return errors.Annotate(errors.Errorf(errCollectorHasRegistered, name), errRegisterCustomCollector)
	}
	return nil
}

type ClusterCollector struct {
	descs []*prometheus.Desc
	hosts []string
}

func (i *ClusterCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(i, ch)
}

func (i *ClusterCollector) Collect(ch chan<- prometheus.Metric) {
	for _, desc := range i.descs {
		for index, host := range i.hosts {
			ch <- prometheus.MustNewConstMetric(
				desc,
				prometheus.CounterValue,
				float64(index),
				host,
			)
		}

	}
}
