package utility

import (
	"errors"
	"net"
	"net/http"
	"strings"
)

var errGetServerAddress = "ltick utility: get server address"

func GetClientIP(req *http.Request) string {
	ip := req.Header.Get("X-Real-IP")
	if ip == "" {
		ip = req.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = req.RemoteAddr
		}
	}
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}
	return ip
}
func GetServerAddress() (ip string, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", errors.New(errGetServerAddress + ": " + err.Error())
	}
	// handle err
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return "", errors.New(errGetServerAddress + ": " + err.Error())
		}
		// handle err
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP.String()
			case *net.IPAddr:
				ip = v.IP.String()
			}
		}
	}
	return ip, nil
}
