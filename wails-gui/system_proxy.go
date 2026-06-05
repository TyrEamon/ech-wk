package main

import (
	"net"
	"strings"
)

type proxySnapshot struct {
	HasEnable     bool
	ProxyEnable   uint64
	HasServer     bool
	ProxyServer   string
	HasOverride   bool
	ProxyOverride string
}

func normalizeProxyListen(listen string) string {
	addr := strings.TrimSpace(listen)
	if addr == "" {
		return ""
	}
	if !strings.Contains(addr, ":") {
		return "127.0.0.1:" + addr
	}
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}
