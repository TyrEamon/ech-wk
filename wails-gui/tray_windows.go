//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	trayUID        = 100
	trayCallback   = 0x8000 + 42
	cmdShowWindow  = 1001
	cmdToggleProxy = 1002
	cmdModeGlobal  = 1003
	cmdModeBypass  = 1004
	cmdModeNone    = 1005
	cmdExitApp     = 1006

	wmCommand       = 0x0111
	wmContextMenu   = 0x007b
	wmClose         = 0x0010
	wmDestroy       = 0x0002
	wmNull          = 0x0000
	wmRButtonUp     = 0x0205
	wmLButtonDblClk = 0x0203
	ninSelect       = 0x0400
	ninKeySelect    = 0x0401

	nimAdd        = 0x00000000
	nimDelete     = 0x00000002
	nimSetVersion = 0x00000004
	nifMessage    = 0x00000001
	nifIcon       = 0x00000002
	nifTip        = 0x00000004
	notifyV4      = 4

	imageIcon      = 1
	lrLoadFromFile = 0x00000010
	lrDefaultSize  = 0x00000040
	idiApplication = 32512

	mfString    = 0x00000000
	mfGrayed    = 0x00000001
	mfChecked   = 0x00000008
	mfSeparator = 0x00000800
	tpmRightBtn = 0x00000002
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	shell32                 = syscall.NewLazyDLL("shell32.dll")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procLoadImageW          = user32.NewProc("LoadImageW")
	procLoadIconW           = user32.NewProc("LoadIconW")
	procDestroyIcon         = user32.NewProc("DestroyIcon")
	procShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")

	activeTrayMu sync.Mutex
	activeTray   *trayManager
)

type trayManager struct {
	app       *App
	hwnd      windows.Handle
	hIcon     windows.Handle
	className *uint16
	wndProc   uintptr
	ready     chan struct{}
	closeOnce sync.Once
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     windows.Handle
	hIcon         windows.Handle
	hCursor       windows.Handle
	hbrBackground windows.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       windows.Handle
}

type point struct {
	x int32
	y int32
}

type msg struct {
	hwnd    windows.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type notifyIconData struct {
	cbSize           uint32
	hwnd             windows.Handle
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            windows.Handle
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         windows.GUID
	hBalloonIcon     windows.Handle
}

func (a *App) setupTray() {
	manager := &trayManager{
		app:     a,
		ready:   make(chan struct{}),
		wndProc: syscall.NewCallback(trayWindowProc),
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
	defer close(t.ready)

	hInstance, _, _ := procGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString("ECHWorkersTrayWindow")
	t.className = className
	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   t.wndProc,
		hInstance:     windows.Handle(hInstance),
		lpszClassName: className,
	}
	if atom, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		t.app.addLog("ERROR", "托盘初始化失败：注册窗口类失败。", "error")
		return
	}

	windowName, _ := syscall.UTF16PtrFromString("ECH Workers Tray")
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		0,
		0, 0, 0, 0,
		0, 0,
		hInstance,
		0,
	)
	if hwnd == 0 {
		t.app.addLog("ERROR", "托盘初始化失败：创建窗口失败。", "error")
		return
	}
	t.hwnd = windows.Handle(hwnd)

	activeTrayMu.Lock()
	activeTray = t
	activeTrayMu.Unlock()

	if err := t.addIcon(); err != nil {
		t.app.addLog("ERROR", fmt.Sprintf("托盘图标加载失败：%v。", err), "error")
	}

	var message msg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func (t *trayManager) close() {
	t.closeOnce.Do(func() {
		select {
		case <-t.ready:
		case <-time.After(2 * time.Second):
		}
		if t.hwnd != 0 {
			procPostMessageW.Call(uintptr(t.hwnd), wmClose, 0, 0)
		}
	})
}

func (t *trayManager) addIcon() error {
	icon, err := loadTrayIcon()
	if err != nil {
		return err
	}
	t.hIcon = icon
	data := notifyIconData{
		cbSize:           uint32(unsafe.Sizeof(notifyIconData{})),
		hwnd:             t.hwnd,
		uID:              trayUID,
		uFlags:           nifMessage | nifIcon | nifTip,
		uCallbackMessage: trayCallback,
		hIcon:            icon,
		uVersion:         notifyV4,
	}
	copyUTF16(data.szTip[:], "ECH Workers")
	if ok, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&data))); ok == 0 {
		return err
	}
	procShellNotifyIconW.Call(nimSetVersion, uintptr(unsafe.Pointer(&data)))
	return nil
}

func (t *trayManager) removeIcon() {
	if t.hwnd != 0 {
		data := notifyIconData{
			cbSize: uint32(unsafe.Sizeof(notifyIconData{})),
			hwnd:   t.hwnd,
			uID:    trayUID,
		}
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&data)))
	}
	if t.hIcon != 0 {
		procDestroyIcon.Call(uintptr(t.hIcon))
		t.hIcon = 0
	}
}

func (t *trayManager) showMenu() {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	state := t.app.state()
	server := t.app.currentServer()
	mode := "bypass_cn"
	if server != nil && server.RoutingMode != "" {
		mode = server.RoutingMode
	}
	t.appendMenu(menu, cmdShowWindow, "显示窗口", 0)
	if state.Running {
		t.appendMenu(menu, cmdToggleProxy, "停止代理", 0)
	} else {
		t.appendMenu(menu, cmdToggleProxy, "启动代理", 0)
	}
	procAppendMenuW.Call(menu, mfSeparator, 0, 0)
	t.appendMenu(menu, cmdModeGlobal, "全局", checkedFlag(mode == "global"))
	t.appendMenu(menu, cmdModeBypass, "绕过大陆", checkedFlag(mode == "bypass_cn"))
	t.appendMenu(menu, cmdModeNone, "不改变", checkedFlag(mode == "none"))
	procAppendMenuW.Call(menu, mfSeparator, 0, 0)
	t.appendMenu(menu, cmdExitApp, "退出", 0)

	var cursor point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
	procSetForegroundWindow.Call(uintptr(t.hwnd))
	procTrackPopupMenu.Call(menu, tpmRightBtn, uintptr(cursor.x), uintptr(cursor.y), 0, uintptr(t.hwnd), 0)
	procPostMessageW.Call(uintptr(t.hwnd), wmNull, 0, 0)
}

func (t *trayManager) appendMenu(menu uintptr, id uint16, label string, flags uintptr) {
	text, _ := syscall.UTF16PtrFromString(label)
	procAppendMenuW.Call(menu, mfString|flags, uintptr(id), uintptr(unsafe.Pointer(text)))
}

func (t *trayManager) handleCommand(id uint16) {
	switch id {
	case cmdShowWindow:
		t.app.showWindow()
	case cmdToggleProxy:
		if t.app.state().Running {
			if _, err := t.app.StopProxy(); err != nil {
				t.app.addLog("ERROR", err.Error(), "error")
			}
		} else if _, err := t.app.StartProxy(); err != nil {
			t.app.addLog("ERROR", err.Error(), "error")
		}
	case cmdModeGlobal:
		t.app.setRoutingModeFromTray("global")
	case cmdModeBypass:
		t.app.setRoutingModeFromTray("bypass_cn")
	case cmdModeNone:
		t.app.setRoutingModeFromTray("none")
	case cmdExitApp:
		t.app.quitApplication()
	}
}

func trayWindowProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	activeTrayMu.Lock()
	tray := activeTray
	activeTrayMu.Unlock()
	if tray == nil {
		ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
		return ret
	}
	switch message {
	case wmContextMenu:
		tray.showMenu()
		return 0
	case trayCallback:
		switch uint32(lParam) {
		case wmLButtonDblClk, ninSelect, ninKeySelect:
			go tray.app.showWindow()
		case wmRButtonUp, wmContextMenu:
			tray.showMenu()
		}
		return 0
	case wmCommand:
		go tray.handleCommand(uint16(wParam & 0xffff))
		return 0
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		tray.removeIcon()
		procPostQuitMessage.Call(0)
		return 0
	default:
		ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
		return ret
	}
}

func loadTrayIcon() (windows.Handle, error) {
	if path := findTrayIconPath(); path != "" {
		ptr, _ := syscall.UTF16PtrFromString(path)
		handle, _, err := procLoadImageW.Call(
			0,
			uintptr(unsafe.Pointer(ptr)),
			imageIcon,
			0,
			0,
			lrLoadFromFile|lrDefaultSize,
		)
		if handle != 0 {
			return windows.Handle(handle), nil
		}
		return 0, err
	}
	handle, _, err := procLoadIconW.Call(0, idiApplication)
	if handle == 0 {
		return 0, err
	}
	return windows.Handle(handle), nil
}

func findTrayIconPath() string {
	exe, _ := os.Executable()
	base := filepath.Dir(exe)
	candidates := []string{
		filepath.Join(base, "app_icon.ico"),
		filepath.Join(base, "icon.ico"),
		filepath.Join(base, "..", "app_icon.ico"),
		filepath.Join(base, "..", "..", "app_icon.ico"),
		filepath.Join(".", "app_icon.ico"),
		filepath.Join("..", "app_icon.ico"),
	}
	for _, candidate := range candidates {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if info, err := os.Stat(abs); err == nil && !info.IsDir() {
			return abs
		}
	}
	return ""
}

func copyUTF16(dst []uint16, text string) {
	encoded := syscall.StringToUTF16(text)
	if len(encoded) > len(dst) {
		encoded = encoded[:len(dst)]
		encoded[len(encoded)-1] = 0
	}
	copy(dst, encoded)
}

func checkedFlag(checked bool) uintptr {
	if checked {
		return mfChecked
	}
	return 0
}
