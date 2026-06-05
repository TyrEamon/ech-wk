package main

type proxySnapshot struct {
	HasEnable     bool
	ProxyEnable   uint64
	HasServer     bool
	ProxyServer   string
	HasOverride   bool
	ProxyOverride string
}
