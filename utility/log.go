package utility

import (
	"context"
	"log"
)

var CustomDefaultLogFunc LogFunc
// LogFunc logs a message using the given format and optional arguments.
// The usage of format and arguments is similar to that for fmt.Printf().
// LogFunc should be thread safe.
type LogFunc func(ctx context.Context, format string, data ...interface{})

func SetDefaultLogFunc(defaultLogFunc LogFunc) {
	CustomDefaultLogFunc = defaultLogFunc
}

func DefaultLogFunc(ctx context.Context, format string, data ...interface{}) {
	if CustomDefaultLogFunc != nil {
		CustomDefaultLogFunc(ctx, format, data...)
	} else {
		log.Printf(format, data...)
	}
}

func DiscardLogFunc(ctx context.Context, format string, data ...interface{}) {
}
