package utility

import (
	"context"
	"log"
)

// LogFunc logs a message using the given format and optional arguments.
// The usage of format and arguments is similar to that for fmt.Printf().
// LogFunc should be thread safe.
type LogFunc func(ctx context.Context, format string, data ...interface{})

func (this *Instance) SetDefaultLogFunc(defaultLogFunc LogFunc) {
	this.CustomDefaultLogFunc = defaultLogFunc
}

func (this *Instance) DefaultLogFunc(ctx context.Context, format string, data ...interface{}) {
	if this.CustomDefaultLogFunc != nil {
		this.CustomDefaultLogFunc(ctx, format, data...)
	} else {
		log.Printf(format, data...)
	}
}

func (this *Instance) DiscardLogFunc(ctx context.Context, format string, data ...interface{}) {
}
