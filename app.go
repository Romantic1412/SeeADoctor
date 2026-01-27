package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"QuickDoctor/core"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx        context.Context
	client     *core.HealthClient
	clientErr  error
	qrMu       sync.Mutex
	qrCancel   context.CancelFunc
	qrToken    uint64
	grabMu     sync.Mutex
	grabCancel context.CancelFunc
	grabToken  uint64
}

type LogEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	client, err := core.NewHealthClient()
	if err != nil {
		a.clientErr = err
		return
	}
	a.client = client
	a.client.EnsureCookiesLoaded()
}

func (a *App) GetCities() ([]map[string]any, error) {
	configDir, err := core.ConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(configDir, "cities.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cities []map[string]any
	if err := json.Unmarshal(data, &cities); err != nil {
		return nil, err
	}
	return cities, nil
}

func (a *App) GetUserState() (map[string]any, error) {
	return core.LoadUserState()
}

func (a *App) SaveUserState(state map[string]any) error {
	return core.SaveUserState(state)
}

func (a *App) LoadGrabConfig() (map[string]any, error) {
	state, err := core.LoadUserState()
	if err != nil {
		return nil, err
	}
	// Extract grab-related configuration
	config := map[string]any{
		"unit_id":              state["unit_id"],
		"unit_name":            state["unit_name"],
		"dep_id":               state["dep_id"],
		"dep_name":             state["dep_name"],
		"doctor_id":            state["doctor_id"],
		"doctor_name":          state["doctor_name"],
		"member_id":            state["member_id"],
		"target_dates":         state["target_dates"],
		"preferred_hours":      state["preferred_hours"],
		"schedule_id":          state["schedule_id"],
		"time_types":           state["time_types"],
		"proxy_submit_enabled": state["proxy_submit_enabled"],
	}
	return config, nil
}

func (a *App) SaveGrabConfig(config map[string]any) error {
	state, err := core.LoadUserState()
	if err != nil {
		return err
	}
	// Merge grab configuration into state
	if val, ok := config["unit_id"]; ok {
		state["unit_id"] = val
	}
	if val, ok := config["unit_name"]; ok {
		state["unit_name"] = val
	}
	if val, ok := config["dep_id"]; ok {
		state["dep_id"] = val
	}
	if val, ok := config["dep_name"]; ok {
		state["dep_name"] = val
	}
	if val, ok := config["doctor_id"]; ok {
		state["doctor_id"] = val
	}
	if val, ok := config["doctor_name"]; ok {
		state["doctor_name"] = val
	}
	if val, ok := config["member_id"]; ok {
		state["member_id"] = val
	}
	if val, ok := config["target_dates"]; ok {
		state["target_dates"] = val
	}
	if val, ok := config["preferred_hours"]; ok {
		state["preferred_hours"] = val
	}
	if val, ok := config["schedule_id"]; ok {
		state["schedule_id"] = val
	}
	if val, ok := config["time_types"]; ok {
		state["time_types"] = val
	}
	if val, ok := config["proxy_submit_enabled"]; ok {
		state["proxy_submit_enabled"] = val
	}
	return core.SaveUserState(state)
}

func (a *App) ExportLogs(entries []LogEntry) (string, error) {
	if a.ctx == nil {
		return "", errors.New("context not ready")
	}
	if len(entries) == 0 {
		return "", errors.New("log entries is empty")
	}

	filename := fmt.Sprintf("quickdoctor_logs_%s.txt", time.Now().Format("20060102_150405"))
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                "导出日志",
		DefaultFilename:      filename,
		CanCreateDirectories: true,
		Filters: []runtime.FileFilter{
			{DisplayName: "Text Files (*.txt)", Pattern: "*.txt"},
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}

	if ext := strings.ToLower(filepath.Ext(path)); ext == "" {
		path += ".txt"
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}

	if strings.EqualFold(filepath.Ext(path), ".json") {
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return "", err
		}
		return path, nil
	}

	var builder strings.Builder
	builder.WriteString("QuickDoctor Logs Export\n")
	builder.WriteString(fmt.Sprintf("ExportedAt: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	builder.WriteString(fmt.Sprintf("Total: %d\n\n", len(entries)))
	for _, entry := range entries {
		level := strings.ToUpper(strings.TrimSpace(entry.Level))
		if level == "" {
			level = "INFO"
		}
		builder.WriteString(fmt.Sprintf("[%s] [%s] %s\n", entry.Time, level, entry.Message))
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (a *App) GetHospitalsByCity(cityID string) ([]map[string]any, error) {
	if _, err := a.ensureSession(); err != nil {
		return nil, err
	}
	return a.client.GetHospitalsByCity(cityID)
}

func (a *App) GetDepsByUnit(unitID string) ([]map[string]any, error) {
	if _, err := a.ensureSession(); err != nil {
		return nil, err
	}
	return a.client.GetDepsByUnit(unitID)
}

func (a *App) GetMembers() ([]core.Member, error) {
	if _, err := a.ensureSession(); err != nil {
		return nil, err
	}
	return a.client.GetMembers()
}

func (a *App) CheckLogin() (bool, error) {
	loaded, err := a.ensureSession()
	if err != nil {
		a.emitLog("error", "登录校验失败：客户端初始化异常")
		return false, err
	}
	if !loaded && !a.client.HasAccessHash() {
		a.emitLog("warn", "登录校验：未发现本地 Cookie")
	}
	if !a.client.HasAccessHash() {
		a.emitLog("warn", "登录校验：缺少 access_hash")
		return false, nil
	}
	ok := a.client.CheckLogin()
	if ok {
		a.emitLog("success", "登录校验通过")
	} else {
		a.emitLog("warn", "登录校验失败")
	}
	return ok, nil
}

func (a *App) GetSchedule(unitID, depID, date string) ([]map[string]any, error) {
	if _, err := a.ensureSession(); err != nil {
		return nil, err
	}
	return a.client.GetSchedule(unitID, depID, date)
}

func (a *App) GetTicketDetail(unitID, depID, scheduleID, memberID string) (*core.TicketDetail, error) {
	if _, err := a.ensureSession(); err != nil {
		return nil, err
	}
	return a.client.GetTicketDetail(unitID, depID, scheduleID, memberID)
}

func (a *App) SubmitOrder(params map[string]any) (*core.SubmitOrderResult, error) {
	if _, err := a.ensureSession(); err != nil {
		return nil, err
	}
	return a.client.SubmitOrder(params)
}

func (a *App) StartQRLogin() error {
	if err := a.ensureClient(); err != nil {
		return err
	}

	a.qrMu.Lock()
	if a.qrCancel != nil {
		a.qrCancel()
	}
	a.qrToken++
	token := a.qrToken
	ctx, cancel := context.WithCancel(context.Background())
	a.qrCancel = cancel
	a.qrMu.Unlock()

	go a.runQRLogin(ctx, token)
	return nil
}

func (a *App) StopQRLogin() {
	a.qrMu.Lock()
	if a.qrCancel != nil {
		a.qrCancel()
		a.qrCancel = nil
	}
	a.qrMu.Unlock()
}

func (a *App) StartGrab(config map[string]any) error {
	if _, err := a.ensureSession(); err != nil {
		return err
	}
	if !a.client.HasAccessHash() {
		a.emitLog("error", "缺少 access_hash，无法启动抢号")
		a.emitLoginStatus(false)
		return fmt.Errorf("%w: 请先扫码登录", core.ErrLoginRequired)
	}
	a.emitLog("info", "检测到 access_hash，允许启动抢号")

	a.grabMu.Lock()
	if a.grabCancel != nil {
		a.grabCancel()
	}
	a.grabToken++
	token := a.grabToken
	ctx, cancel := context.WithCancel(context.Background())
	a.grabCancel = cancel
	a.grabMu.Unlock()

	go a.runGrab(ctx, token, config)
	return nil
}

func (a *App) StopGrab() {
	a.grabMu.Lock()
	if a.grabCancel != nil {
		a.grabCancel()
		a.grabCancel = nil
	}
	a.grabMu.Unlock()
}

func (a *App) ensureClient() error {
	if a.client != nil {
		return nil
	}
	if a.clientErr != nil {
		return a.clientErr
	}
	client, err := core.NewHealthClient()
	if err != nil {
		a.clientErr = err
		return err
	}
	a.client = client
	a.client.EnsureCookiesLoaded()
	return nil
}

func (a *App) ensureSession() (bool, error) {
	if err := a.ensureClient(); err != nil {
		return false, err
	}
	if a.client == nil {
		return false, errors.New("client not ready")
	}
	loaded := a.client.EnsureCookiesLoaded()
	return loaded, nil
}

func (a *App) runQRLogin(ctx context.Context, token uint64) {
	defer func() {
		a.qrMu.Lock()
		if a.qrToken == token {
			a.qrCancel = nil
		}
		a.qrMu.Unlock()
	}()

	login, err := core.NewFastQRLogin()
	if err != nil {
		a.emitLog("error", "二维码登录初始化失败: "+err.Error())
		a.emitQRStatus("二维码登录初始化失败")
		return
	}

	a.emitQRStatus("正在获取二维码...")
	qrBytes, uuid, err := login.GetQRImage()
	if err != nil {
		a.emitLog("error", "获取二维码失败: "+err.Error())
		a.emitQRStatus("获取二维码失败")
		return
	}
	payload := map[string]string{
		"uuid":   uuid,
		"base64": base64.StdEncoding.EncodeToString(qrBytes),
	}
	runtime.EventsEmit(a.ctx, "qr-image", payload)
	a.emitQRStatus("请使用微信扫码")

	result := login.PollStatus(ctx, 5*time.Minute, func(msg string) {
		a.emitQRStatus(translateQRStatus(msg))
	})

	if result.Success {
		a.emitLog("success", "登录成功")
		a.emitLoginStatus(true)
		if a.client != nil {
			a.client.LoadCookies()
		}
	} else {
		a.emitLog("error", "登录失败: "+translateQRError(result.Message))
		a.emitLoginStatus(false)
	}
}

func (a *App) runGrab(ctx context.Context, token uint64, config map[string]any) {
	defer func() {
		a.grabMu.Lock()
		if a.grabToken == token {
			a.grabCancel = nil
		}
		a.grabMu.Unlock()
	}()

	grabber := core.NewGrabber(a.client)
	result := grabber.Run(ctx, config, func(level, message string) {
		a.emitLog(level, message)
	})

	if ctx.Err() != nil {
		a.emitGrabFinished(false, "stopped", nil)
		return
	}
	if errors.Is(result.Err, core.ErrLoginRequired) {
		a.emitLoginStatus(false)
	}
	if result.Success {
		a.emitGrabFinished(true, result.Message, result.Detail)
		return
	}
	a.emitGrabFinished(false, result.Message, nil)
}

func (a *App) emitLog(level, message string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "log-message", map[string]string{
		"level":   level,
		"message": message,
	})
}

func (a *App) emitQRStatus(message string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "qr-status", map[string]string{
		"message": message,
	})
}

func (a *App) emitLoginStatus(loggedIn bool) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "login-status", map[string]any{
		"loggedIn": loggedIn,
	})
}

func (a *App) emitGrabFinished(success bool, message string, detail *core.GrabSuccess) {
	if a.ctx == nil {
		return
	}
	payload := map[string]any{
		"success": success,
		"message": message,
	}
	if detail != nil {
		payload["detail"] = detail
	}
	runtime.EventsEmit(a.ctx, "grab-finished", payload)
}

func translateQRStatus(message string) string {
	switch message {
	case "waiting for scan":
		return "等待扫码..."
	case "scanned, confirm on phone":
		return "已扫码，请在手机上确认"
	case "logging in":
		return "正在登录..."
	case "confirmed but no code, retrying":
		return "已确认但未获取到登录码，正在重试..."
	default:
		return message
	}
}

func translateQRError(message string) string {
	switch message {
	case "canceled":
		return "已取消"
	case "qr expired":
		return "二维码已过期"
	case "uuid not initialized":
		return "二维码未初始化"
	case "no cookies received":
		return "未获取到有效 Cookie"
	case "missing access_hash":
		return "登录未完成：缺少 access_hash"
	default:
		return message
	}
}

func (a *App) Greet(name string) string {
	if name == "" {
		name = "friend"
	}
	return "Hello " + name + ", It's show time!"
}
