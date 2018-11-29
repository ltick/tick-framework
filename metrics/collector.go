package metrics

import (
	"context"

	"github.com/ltick/tick-routing"
	"github.com/prometheus/client_golang/prometheus"
)

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
func (i *Collector) OnStartup(c *routing.Context) error {
	return nil
}
func (i *Collector) OnShutdown(c *routing.Context) error {
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
