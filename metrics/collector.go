package metrics

import (
	"context"

	"github.com/juju/errors"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	errCollectorHasBeenRegistered = "metrics: collector has been registered"
	errCollector                  = "metrics: register collector"
)

var collectors map[string]prometheus.Collector = make(map[string]prometheus.Collector, 0)

type Collector struct {
	ClusterCollector *ClusterCollector
}

func NewCollector(reg prometheus.Registerer, descs []*prometheus.Desc, zone string, hosts []string) *Collector {
	c := &ClusterCollector{
		descs: descs,
		hosts: hosts,
	}
	cc := &Collector{
		ClusterCollector: c,
	}
	prometheus.WrapRegistererWith(prometheus.Labels{"zone": zone}, reg).MustRegister(c)
	reg.MustRegister(
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		prometheus.NewGoCollector(),
	)
	return cc
}

func (i *Collector) Prepare(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (i *Collector) Initiate(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (i *Collector) OnStartup(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (i *Collector) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func (i *Collector) RegisterCollector(name string, cs prometheus.Collector) error {
	if _, ok := collectors[name]; !ok {
		collectors[name] = cs
		prometheus.MustRegister(cs)
	} else {
		return errors.New(errCollectorHasBeenRegistered)
	}
	return nil
}
func (i *Collector) GetCollector(name string) (cs prometheus.Collector) {
	if _, ok := collectors[name]; ok {
		return collectors[name]
	} else {
		return nil
	}
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
