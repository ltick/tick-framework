package config

import (
	"context"
	"strings"
)

var Realpath = func(ctx context.Context, value interface{}) (interface{}, error) {
	ctxPathPrefix := ctx.Value("PATH_PREFIX")
	if ctxPathPrefix != nil {
		if prefixPath, ok := ctxPathPrefix.(string); ok {
			if path, ok := value.(string); ok {
				if path != "" {
					if !strings.HasPrefix(path, "/") {
						return strings.TrimRight(prefixPath, "/") + "/" + path, nil
					}
				}
			}
		}
	}
	return value, nil
}