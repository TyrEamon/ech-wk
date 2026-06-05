//go:build windows

package main

import (
	"errors"
	"fmt"
	"syscall"

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
	if snap.HasEnable {
		if err := key.SetDWordValue("ProxyEnable", uint32(snap.ProxyEnable)); err != nil {
			return err
		}
	} else if err := key.SetDWordValue("ProxyEnable", 0); err != nil {
		return err
	}
	if err := restoreStringValue(key, "ProxyServer", snap.HasServer, snap.ProxyServer); err != nil {
		return err
	}
	if err := restoreStringValue(key, "ProxyOverride", snap.HasOverride, snap.ProxyOverride); err != nil {
		return err
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
