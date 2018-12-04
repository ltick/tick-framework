package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/ltick/tick-routing"
)

type IPFilter struct {
	whitelist []string
	realIP    bool
}

// NewIPFilter creates middleware that intercepts the specified IP prefix.
func NewIPFilter(whitelist []string, realIP bool) *IPFilter {
	return &IPFilter{
		whitelist: whitelist,
		realIP:    realIP,
	}
}
func (i *IPFilter) Prepare(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (i *IPFilter) Initiate(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (i *IPFilter) OnRequestStartup(c *routing.Context) error {
	var forbidden bool
	var match []string
	var prefix []string
	if len(i.whitelist) == 0 {
		forbidden = true
	} else {
		for _, s := range i.whitelist {
			if strings.HasSuffix(s, "*") {
				prefix = append(prefix, s[:len(s)-1])
			} else {
				match = append(match, s)
			}
		}
	}
	if forbidden {
		return routing.NewHTTPError(http.StatusForbidden)
	}
	var ip string
	if i.realIP {
		ip = c.GetClientRealIP()
	} else {
		ip = c.GetClientRemoteIP()
	}
	for _, ipMatch := range match {
		if ipMatch == ip {
			c.Next()
			return nil
		}
	}
	for _, ipPrefix := range prefix {
		if strings.HasPrefix(ip, ipPrefix) {
			c.Next()
			return nil
		}
	}
	return routing.NewHTTPError(http.StatusForbidden, "access not allow for ip: "+ip)
}
func (i *IPFilter) OnRequestShutdown(c *routing.Context) error {
	return nil
}
