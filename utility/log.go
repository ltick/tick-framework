package utility

import (
	"context"
)

// LogFunc logs a message using the given format and optional arguments.
// The usage of format and arguments is similar to that for fmt.Printf().
// LogFunc should be thread safe.
type LogFunc func(ctx context.Context, format string, data ...interface{})

func DiscardLogFunc(ctx context.Context, format string, data ...interface{}) {
}

