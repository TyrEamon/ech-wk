//go:build windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/getlantern/systray"
)

type trayManager struct {
	app        *App
	ready      chan struct{}
	done       chan struct{}
	closeOnce  sync.Once
	doneOnce   sync.Once
	showItem   *systray.MenuItem
	toggleItem *systray.MenuItem
	globalItem *systray.MenuItem
	bypassItem *systray.MenuItem
	noneItem   *systray.MenuItem
	quitItem   *systray.MenuItem
}

func (a *App) setupTray() {
	manager := &trayManager{
		app:   a,
		ready: make(chan struct{}),
		done:  make(chan struct{}),
	}
	a.tray = manager
	go manager.run()
}

func (a *App) cleanupTray() {
	if a.tray != nil {
		a.tray.close()
	}
}

func (t *trayManager) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	systray.Run(t.onReady, t.onExit)
}

func (t *trayManager) onReady() {
	if icon, err := readTrayIcon(); err == nil {
		systray.SetIcon(icon)
	}
	systray.SetTooltip("ECH Workers")

	t.showItem = systray.AddMenuItem("显示窗口", "显示 ECH Workers")
	t.toggleItem = systray.AddMenuItem("启动代理", "启动或停止代理")
	systray.AddSeparator()
	t.globalItem = systray.AddMenuItem("全局", "所有流量通过代理")
	t.bypassItem = systray.AddMenuItem("绕过大陆", "大陆流量直连")
	t.noneItem = systray.AddMenuItem("不改变", "不修改系统代理")
	systray.AddSeparator()
	t.quitItem = systray.AddMenuItem("退出", "退出 ECH Workers")

	t.refresh()
	close(t.ready)
	go t.watchClicks()
}

func (t *trayManager) onExit() {
	t.doneOnce.Do(func() {
		close(t.done)
	})
}

func (t *trayManager) close() {
	t.closeOnce.Do(func() {
		systray.Quit()
		select {
		case <-t.done:
		case <-time.After(2 * time.Second):
		}
	})
}

func (t *trayManager) watchClicks() {
	ticker := time.NewTicker(900 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-t.showItem.ClickedCh:
			t.app.showWindow()
		case <-t.toggleItem.ClickedCh:
			t.toggleProxy()
		case <-t.globalItem.ClickedCh:
			t.app.setRoutingModeFromTray("global")
			t.refresh()
		case <-t.bypassItem.ClickedCh:
			t.app.setRoutingModeFromTray("bypass_cn")
			t.refresh()
		case <-t.noneItem.ClickedCh:
			t.app.setRoutingModeFromTray("none")
			t.refresh()
		case <-t.quitItem.ClickedCh:
			t.app.quitApplication()
		case <-ticker.C:
			t.refresh()
		case <-t.done:
			return
		}
	}
}

func (t *trayManager) toggleProxy() {
	if t.app.state().Running {
		if _, err := t.app.StopProxy(); err != nil {
			t.app.addLog("ERROR", err.Error(), "error")
		}
	} else if _, err := t.app.StartProxy(); err != nil {
		t.app.addLog("ERROR", err.Error(), "error")
	}
	t.refresh()
}

func (t *trayManager) refresh() {
	if t.toggleItem == nil {
		return
	}
	state := t.app.state()
	if state.Running {
		t.toggleItem.SetTitle("停止代理")
	} else {
		t.toggleItem.SetTitle("启动代理")
	}

	mode := "bypass_cn"
	if server := t.app.currentServer(); server != nil && server.RoutingMode != "" {
		mode = server.RoutingMode
	}
	setChecked(t.globalItem, mode == "global")
	setChecked(t.bypassItem, mode == "bypass_cn")
	setChecked(t.noneItem, mode == "none")
}

func setChecked(item *systray.MenuItem, checked bool) {
	if item == nil {
		return
	}
	if checked {
		item.Check()
	} else {
		item.Uncheck()
	}
}

func readTrayIcon() ([]byte, error) {
	for _, candidate := range trayIconCandidates() {
		if data, err := os.ReadFile(candidate); err == nil {
			return data, nil
		}
	}
	return nil, os.ErrNotExist
}

func trayIconCandidates() []string {
	exe, _ := os.Executable()
	base := filepath.Dir(exe)
	absRoot, _ := filepath.Abs(".")
	return []string{
		filepath.Join(base, "app_icon.ico"),
		filepath.Join(base, "icon.ico"),
		filepath.Join(base, "..", "app_icon.ico"),
		filepath.Join(base, "..", "..", "app_icon.ico"),
		filepath.Join(absRoot, "app_icon.ico"),
		filepath.Join(absRoot, "..", "app_icon.ico"),
		filepath.Join(absRoot, "build", "windows", "icon.ico"),
	}
}
