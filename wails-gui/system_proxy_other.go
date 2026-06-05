//go:build !windows

package main

import (
	"fmt"
	"runtime"
)

func setSystemProxy(enabled bool, listen, routingMode string, snapshot **proxySnapshot) error {
	if !enabled {
		*snapshot = nil
		return nil
	}
	return fmt.Errorf("%s 暂不支持自动设置系统代理", runtime.GOOS)
}
