//go:build windows

package main

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows/registry"
)

const internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

func setSystemProxy(enabled bool, listen, routingMode string, snapshot **proxySnapshot) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if enabled {
		if *snapshot == nil {
			snap := &proxySnapshot{}
			if value, _, err := key.GetIntegerValue("ProxyEnable"); err == nil {
				snap.HasEnable = true
				snap.ProxyEnable = value
			}
			if value, _, err := key.GetStringValue("ProxyServer"); err == nil {
				snap.HasServer = true
				snap.ProxyServer = value
			}
			if value, _, err := key.GetStringValue("ProxyOverride"); err == nil {
				snap.HasOverride = true
				snap.ProxyOverride = value
			}
			*snapshot = snap
		}

		proxyServer := normalizeProxyListen(listen)
		if proxyServer == "" {
			return fmt.Errorf("监听地址为空")
		}
		if err := key.SetStringValue("ProxyServer", proxyServer); err != nil {
			return err
		}
		if err := key.SetDWordValue("ProxyEnable", 1); err != nil {
			return err
		}
		if err := key.SetStringValue("ProxyOverride", windowsProxyBypassList(routingMode)); err != nil {
			return err
		}
		notifyProxyChanged()
		return nil
	}

	if *snapshot == nil {
		if err := key.SetDWordValue("ProxyEnable", 0); err != nil {
			return err
		}
		notifyProxyChanged()
		return nil
	}

	snap := *snapshot
	restoreSnapshot := isProxySnapshotUsable(snap)
	if restoreSnapshot && snap.HasEnable {
		if err := key.SetDWordValue("ProxyEnable", uint32(snap.ProxyEnable)); err != nil {
			return err
		}
	} else if err := key.SetDWordValue("ProxyEnable", 0); err != nil {
		return err
	}
	if restoreSnapshot {
		if err := restoreStringValue(key, "ProxyServer", snap.HasServer, snap.ProxyServer); err != nil {
			return err
		}
		if err := restoreStringValue(key, "ProxyOverride", snap.HasOverride, snap.ProxyOverride); err != nil {
			return err
		}
	}
	*snapshot = nil
	notifyProxyChanged()
	return nil
}

func restoreStringValue(key registry.Key, name string, exists bool, value string) error {
	if exists {
		return key.SetStringValue(name, value)
	}
	err := key.DeleteValue(name)
	if err == nil || errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	return err
}

func windowsProxyBypassList(routingMode string) string {
	return "localhost;127.*;10.*;172.16.*;172.17.*;172.18.*;172.19.*;172.20.*;172.21.*;172.22.*;172.23.*;172.24.*;172.25.*;172.26.*;172.27.*;172.28.*;172.29.*;172.30.*;172.31.*;192.168.*;<local>"
}

func isProxySnapshotUsable(snap *proxySnapshot) bool {
	if snap == nil || !snap.HasEnable || snap.ProxyEnable == 0 {
		return true
	}
	if !snap.HasServer || strings.TrimSpace(snap.ProxyServer) == "" {
		return false
	}
	endpoints := proxyServerEndpoints(snap.ProxyServer)
	if len(endpoints) == 0 {
		return false
	}
	hasLocalEndpoint := false
	for _, endpoint := range endpoints {
		host, _, err := net.SplitHostPort(endpoint)
		if err != nil {
			return true
		}
		if !isLocalProxyHost(host) {
			return true
		}
		hasLocalEndpoint = true
		if canDialLocalProxy(endpoint) {
			return true
		}
	}
	return !hasLocalEndpoint
}

func proxyServerEndpoints(proxyServer string) []string {
	parts := strings.FieldsFunc(proxyServer, func(r rune) bool {
		return r == ';' || r == ' '
	})
	endpoints := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if key, endpoint, ok := strings.Cut(value, "="); ok && key != "" {
			value = endpoint
		}
		if value == "" {
			continue
		}
		endpoints = append(endpoints, normalizeProxyListen(value))
	}
	return endpoints
}

func isLocalProxyHost(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	return host == "localhost" || host == "::1" || strings.HasPrefix(host, "127.")
}

func canDialLocalProxy(endpoint string) bool {
	conn, err := net.DialTimeout("tcp", endpoint, 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func notifyProxyChanged() {
	wininet := syscall.NewLazyDLL("wininet.dll")
	internetSetOption := wininet.NewProc("InternetSetOptionW")
	const (
		internetOptionRefresh         = 37
		internetOptionSettingsChanged = 39
	)
	internetSetOption.Call(0, uintptr(internetOptionSettingsChanged), 0, 0)
	internetSetOption.Call(0, uintptr(internetOptionRefresh), 0, 0)
}
