package ltick

import (
	"net/http"
	"time"

	"github.com/ltick/tick-routing"
	"github.com/ltick/tick-routing/access"
)

func CustomLogger(loggerFunc access.LogWriterFunc) routing.Handler {
	return func(c *routing.Context) error {
		startTime := time.Now()

		rw := &access.LogResponseWriter{c.ResponseWriter, http.StatusOK, 0}
		c.ResponseWriter = rw

		err := c.Next()

		elapsed := float64(time.Now().Sub(startTime).Nanoseconds()) / 1e6
		loggerFunc(c.Request, rw, elapsed)

		return err
	}
}
