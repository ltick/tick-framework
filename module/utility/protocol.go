package utility

import (
	"context"

	"github.com/ltick/tick-routing"
)

var (
	errInitiate = "utility: initiate error"
)

func NewInstance() *Instance {
	return &Instance{}
}

type Instance struct {
	CustomDefaultLogFunc LogFunc
}

func (this *Instance) Initiate(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (this *Instance) OnStartup(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (this *Instance) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (this *Instance) OnRequestStartup(c *routing.Context) error {
	return nil
}
func (this *Instance) OnRequestShutdown(c *routing.Context) error {
	return nil
}
