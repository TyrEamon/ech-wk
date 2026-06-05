package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type Server struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Server      string `json:"server"`
	Listen      string `json:"listen"`
	Token       string `json:"token"`
	IP          string `json:"ip"`
	DNS         string `json:"dns"`
	ECH         string `json:"ech"`
	RoutingMode string `json:"routing_mode"`
}

type Config struct {
	Servers         []Server `json:"servers"`
	CurrentServerID string   `json:"current_server_id"`
}

type LogEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Tone    string `json:"tone"`
}

type State struct {
	Servers         []Server   `json:"servers"`
	CurrentServerID string     `json:"current_server_id"`
	Running         bool       `json:"running"`
	Logs            []LogEntry `json:"logs"`
}

type App struct {
	ctx        context.Context
	mu         sync.Mutex
	configPath string
	cfg        Config
	logs       []LogEntry
	cmd        *exec.Cmd
}

func NewApp() *App {
	app := &App{}
	app.configPath = app.defaultConfigPath()
	app.loadConfig()
	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.addLog("INFO", "初始化完成。", "")
	if server := a.currentServer(); server != nil {
		a.addLog("INFO", fmt.Sprintf("已载入：%s。", server.Name), "")
	}
	a.emitState()
}

func (a *App) shutdown(ctx context.Context) {
	_ = a.StopProxy()
}

func (a *App) GetState() (State, error) {
	return a.state(), nil
}

func (a *App) SelectServer(id string) (State, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.isRunningLocked() {
		return State{}, errors.New("请先停止当前连接后再切换服务器")
	}
	for _, server := range a.cfg.Servers {
		if server.ID == id {
			a.cfg.CurrentServerID = id
			if err := a.saveConfigLocked(); err != nil {
				return State{}, err
			}
			return a.stateLocked(), nil
		}
	}
	return State{}, errors.New("服务器不存在")
}

func (a *App) SaveServer(server Server) (State, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if strings.TrimSpace(server.Name) == "" {
		server.Name = "未命名服务器"
	}
	if strings.TrimSpace(server.Listen) == "" {
		server.Listen = "127.0.0.1:30000"
	}
	if strings.TrimSpace(server.DNS) == "" {
		server.DNS = "dns.alidns.com/dns-query"
	}
	if strings.TrimSpace(server.ECH) == "" {
		server.ECH = "cloudflare-ech.com"
	}
	if server.RoutingMode == "" {
		server.RoutingMode = "bypass_cn"
	}
	if server.ID == "" {
		server.ID = newID()
		a.cfg.Servers = append(a.cfg.Servers, server)
		a.cfg.CurrentServerID = server.ID
	} else {
		found := false
		for i := range a.cfg.Servers {
			if a.cfg.Servers[i].ID == server.ID {
				a.cfg.Servers[i] = server
				found = true
				break
			}
		}
		if !found {
			a.cfg.Servers = append(a.cfg.Servers, server)
		}
	}
	if a.cfg.CurrentServerID == "" {
		a.cfg.CurrentServerID = server.ID
	}
	if err := a.saveConfigLocked(); err != nil {
		return State{}, err
	}
	a.addLogUnlocked("INFO", fmt.Sprintf("配置已保存：%s。", server.Name), "")
	return a.stateLocked(), nil
}

func (a *App) CreateServer() (State, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	name := a.nextServerNameLocked()
	server := Server{
		ID:          newID(),
		Name:        name,
		Listen:      "127.0.0.1:30000",
		DNS:         "dns.alidns.com/dns-query",
		ECH:         "cloudflare-ech.com",
		RoutingMode: "bypass_cn",
	}
	a.cfg.Servers = append(a.cfg.Servers, server)
	a.cfg.CurrentServerID = server.ID
	if err := a.saveConfigLocked(); err != nil {
		return State{}, err
	}
	a.addLogUnlocked("INFO", fmt.Sprintf("已创建：%s。", name), "")
	return a.stateLocked(), nil
}

func (a *App) DeleteServer(id string) (State, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.cfg.Servers) <= 1 {
		return State{}, errors.New("至少保留一个服务器")
	}
	next := a.cfg.Servers[:0]
	deleted := ""
	for _, server := range a.cfg.Servers {
		if server.ID == id {
			deleted = server.Name
			continue
		}
		next = append(next, server)
	}
	if deleted == "" {
		return State{}, errors.New("服务器不存在")
	}
	a.cfg.Servers = next
	if a.cfg.CurrentServerID == id {
		a.cfg.CurrentServerID = a.cfg.Servers[0].ID
	}
	if err := a.saveConfigLocked(); err != nil {
		return State{}, err
	}
	a.addLogUnlocked("WARN", fmt.Sprintf("已删除：%s。", deleted), "warn")
	return a.stateLocked(), nil
}

func (a *App) SetRoutingMode(mode string) (State, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if mode != "global" && mode != "bypass_cn" && mode != "none" {
		return State{}, errors.New("未知分流模式")
	}
	idx := a.currentServerIndexLocked()
	if idx < 0 {
		return State{}, errors.New("没有可用服务器")
	}
	a.cfg.Servers[idx].RoutingMode = mode
	if err := a.saveConfigLocked(); err != nil {
		return State{}, err
	}
	a.addLogUnlocked("INFO", fmt.Sprintf("模式：%s。", routingLabel(mode)), "")
	return a.stateLocked(), nil
}

func (a *App) StartProxy() (State, error) {
	a.mu.Lock()
	if a.isRunningLocked() {
		state := a.stateLocked()
		a.mu.Unlock()
		return state, nil
	}
	server := a.currentServerLocked()
	if server == nil {
		a.mu.Unlock()
		return State{}, errors.New("没有可用服务器")
	}
	if strings.TrimSpace(server.Server) == "" {
		a.mu.Unlock()
		return State{}, errors.New("请先填写服务地址")
	}
	core, err := findCoreExecutable()
	if err != nil {
		a.mu.Unlock()
		return State{}, err
	}
	args := []string{
		"-l", server.Listen,
		"-f", server.Server,
		"-dns", server.DNS,
		"-ech", server.ECH,
		"-routing", server.RoutingMode,
	}
	if server.Token != "" {
		args = append(args, "-token", server.Token)
	}
	if server.IP != "" {
		args = append(args, "-ip", server.IP)
	}
	cmd := exec.Command(core, args...)
	applyProcessAttrs(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.mu.Unlock()
		return State{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		a.mu.Unlock()
		return State{}, err
	}
	if err := cmd.Start(); err != nil {
		a.mu.Unlock()
		return State{}, err
	}
	a.cmd = cmd
	a.addLogUnlocked("INFO", fmt.Sprintf("已启动：%s。", server.Name), "")
	state := a.stateLocked()
	a.mu.Unlock()

	go a.scanPipe(stdout)
	go a.scanPipe(stderr)
	go a.waitProcess(cmd)
	a.emitState()
	return state, nil
}

func (a *App) StopProxy() (State, error) {
	a.mu.Lock()
	cmd := a.cmd
	if cmd == nil || cmd.Process == nil {
		state := a.stateLocked()
		a.mu.Unlock()
		return state, nil
	}
	a.cmd = nil
	a.mu.Unlock()

	_ = cmd.Process.Kill()
	a.addLog("WARN", "已停止。", "warn")
	a.emitState()
	return a.state(), nil
}

func (a *App) TestLatency() (State, error) {
	start := time.Now()
	client := http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get("https://www.gstatic.com/generate_204")
	elapsed := time.Since(start)
	if err != nil {
		a.addLog("ERROR", fmt.Sprintf("测速失败：%v。", err), "error")
		return a.state(), err
	}
	defer resp.Body.Close()
	a.addLog("INFO", fmt.Sprintf("真实链路：%dms。", elapsed.Milliseconds()), "")
	return a.state(), nil
}

func (a *App) ClearLogs() (State, error) {
	a.mu.Lock()
	a.logs = nil
	a.mu.Unlock()
	a.addLog("INFO", "日志已清空。", "")
	return a.state(), nil
}

func (a *App) state() State {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stateLocked()
}

func (a *App) stateLocked() State {
	servers := append([]Server(nil), a.cfg.Servers...)
	logs := append([]LogEntry(nil), a.logs...)
	return State{
		Servers:         servers,
		CurrentServerID: a.cfg.CurrentServerID,
		Running:         a.isRunningLocked(),
		Logs:            logs,
	}
}

func (a *App) currentServer() *Server {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.currentServerLocked()
}

func (a *App) currentServerLocked() *Server {
	idx := a.currentServerIndexLocked()
	if idx < 0 {
		return nil
	}
	server := a.cfg.Servers[idx]
	return &server
}

func (a *App) currentServerIndexLocked() int {
	if len(a.cfg.Servers) == 0 {
		return -1
	}
	for i := range a.cfg.Servers {
		if a.cfg.Servers[i].ID == a.cfg.CurrentServerID {
			return i
		}
	}
	a.cfg.CurrentServerID = a.cfg.Servers[0].ID
	return 0
}

func (a *App) isRunningLocked() bool {
	return a.cmd != nil && a.cmd.Process != nil
}

func (a *App) addLog(level, message, tone string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.addLogUnlocked(level, message, tone)
}

func (a *App) addLogUnlocked(level, message, tone string) {
	entry := LogEntry{
		Time:    time.Now().Format("15:04:05"),
		Level:   level,
		Message: message,
		Tone:    tone,
	}
	a.logs = append(a.logs, entry)
	if len(a.logs) > 400 {
		a.logs = a.logs[len(a.logs)-400:]
	}
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "proxy:log", entry)
	}
}

func (a *App) emitState() {
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "proxy:state", a.state())
	}
}

func (a *App) scanPipe(pipe interface{ Read([]byte) (int, error) }) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		level := "INFO"
		tone := ""
		if strings.Contains(line, "[错误]") || strings.Contains(line, "error") || strings.Contains(line, "失败") {
			level = "ERROR"
			tone = "error"
		} else if strings.Contains(line, "[警告]") || strings.Contains(line, "warn") {
			level = "WARN"
			tone = "warn"
		}
		a.addLog(level, line, tone)
	}
}

func (a *App) waitProcess(cmd *exec.Cmd) {
	err := cmd.Wait()
	a.mu.Lock()
	if a.cmd == cmd {
		a.cmd = nil
	}
	a.mu.Unlock()
	if err != nil {
		a.addLog("WARN", fmt.Sprintf("进程退出：%v。", err), "warn")
	} else {
		a.addLog("WARN", "进程已停止。", "warn")
	}
	a.emitState()
}

func (a *App) defaultConfigPath() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr == nil {
			base = home
		}
	}
	return filepath.Join(base, "ECHWorkersClient", "config.json")
}

func (a *App) loadConfig() {
	data, err := os.ReadFile(a.configPath)
	if err == nil {
		_ = json.Unmarshal(data, &a.cfg)
	}
	if len(a.cfg.Servers) == 0 {
		a.cfg.Servers = []Server{{
			ID:          newID(),
			Name:        "默认服务器",
			Listen:      "127.0.0.1:30000",
			DNS:         "dns.alidns.com/dns-query",
			ECH:         "cloudflare-ech.com",
			RoutingMode: "bypass_cn",
		}}
		a.cfg.CurrentServerID = a.cfg.Servers[0].ID
		_ = os.MkdirAll(filepath.Dir(a.configPath), 0755)
		_ = a.saveConfigLocked()
	}
	if a.cfg.CurrentServerID == "" {
		a.cfg.CurrentServerID = a.cfg.Servers[0].ID
	}
}

func (a *App) saveConfigLocked() error {
	if err := os.MkdirAll(filepath.Dir(a.configPath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(a.cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.configPath, data, 0644)
}

func (a *App) nextServerNameLocked() string {
	existing := map[string]bool{}
	for _, server := range a.cfg.Servers {
		existing[server.Name] = true
	}
	for i := 1; ; i++ {
		name := fmt.Sprintf("新服务器%d", i)
		if !existing[name] {
			return name
		}
	}
}

func newID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func routingLabel(mode string) string {
	switch mode {
	case "global":
		return "全局"
	case "bypass_cn":
		return "绕过大陆"
	case "none":
		return "不改变"
	default:
		return mode
	}
}

func findCoreExecutable() (string, error) {
	name := "ech-workers"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	exe, _ := os.Executable()
	candidates := []string{
		filepath.Join(filepath.Dir(exe), name),
		filepath.Join(".", name),
		filepath.Join("..", name),
		filepath.Join("..", "..", name),
		filepath.Join("..", "..", "..", name),
	}
	for _, candidate := range candidates {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if info, err := os.Stat(abs); err == nil && !info.IsDir() {
			return abs, nil
		}
	}
	return "", fmt.Errorf("找不到代理内核 %s，请确认它和 Wails GUI 在同一目录", name)
}
