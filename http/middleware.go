package http

import (
	"context"
	"github.com/ltick/tick-routing"
)

type MiddlewareInterface interface {
	Initiate(ctx context.Context) (context.Context, error)
	OnRequestStartup(c *routing.Context) error
	OnRequestShutdown(c *routing.Context) error
}
