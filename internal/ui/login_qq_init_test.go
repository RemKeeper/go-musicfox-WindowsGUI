package ui

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/skip2/go-qrcode"

	"github.com/go-musicfox/go-musicfox/internal/qqmusic"
)

func buildTestQRPNG(t *testing.T, content string) []byte {
	t.Helper()
	pngBytes, err := qrcode.Encode(content, qrcode.Medium, 128)
	if err != nil {
		t.Fatalf("encode test QR PNG failed: %v", err)
	}
	return pngBytes
}

// TestToQQLoginPageStartsQRGeneration 是「二维码卡在正在获取」的回归测试。
//
// 历史 bug：QQLoginPage 依赖 Init() 生成二维码，但 foxful-cli 的
// InitPage 接口只在 App 启动时对初始页面调用一次（见 model/app.go:167），
// 运行期切换页面不会触发。结果进入登录页后一直停在「正在获取二维码...」，
// 必须手动左右切换 QQ/微信 Tab（走 switchTab 路径）才会渲染。
//
// 修复：ToQQLoginPage 显式返回生成二维码的 cmd。
func TestToQQLoginPageStartsQRGeneration(t *testing.T) {
	app, n, _ := setupSwitchSourceTestApp(t)
	n.SetSource(SourceQQ)

	page, cmd := n.ToQQLoginPage(nil)

	qrPage, ok := page.(*QQLoginPage)
	if !ok {
		t.Fatalf("返回的不是 QQLoginPage: %T", page)
	}

	var calls atomic.Int32
	pngBytes := buildTestQRPNG(t, "https://example.com/qq")
	qrPage.fetchQR = func(loginType qqmusic.QRLoginType) (*qqmusic.QRCode, error) {
		calls.Add(1)
		return &qqmusic.QRCode{
			Image:      pngBytes,
			MimeType:   "image/png",
			Identifier: "test-qrsig",
			Type:       loginType,
		}, nil
	}

	// 初始文案应为获取中
	if qrPage.statusMsg != "正在获取二维码..." {
		t.Errorf("初始 statusMsg = %q, want \"正在获取二维码...\"", qrPage.statusMsg)
	}
	if qrPage.qrView != "" {
		t.Error("初始 qrView 应为空")
	}

	// 关键断言：必须返回可执行的 cmd，否则二维码永远不会生成
	if cmd == nil {
		t.Fatal("ToQQLoginPage 必须返回生成二维码的 cmd，" +
			"否则页面会卡在「正在获取二维码...」")
	}

	// 执行 cmd，验证确实产出了二维码消息
	// （tea.Batch 会返回 BatchMsg，逐个执行其中的子命令）
	msg := cmd()
	generated := findQRGeneratedMsg(msg)
	if generated == nil {
		t.Fatalf("cmd 未产出 qqQRGeneratedMsg，实际类型 %T", msg)
	}
	if generated.err != nil {
		t.Fatalf("生成二维码失败: %v", generated.err)
	}
	if generated.qrView == "" {
		t.Error("生成的 qrView 为空，无法渲染")
	}
	if generated.qr == nil {
		t.Error("生成的 qr 为 nil")
	}

	// 已执行过 cmd 后，后续收到 tickLoginMsg 不应重复发起请求。
	_, followUpCmd := qrPage.Update(qrPage.Msg(), app)
	if followUpCmd != nil {
		if m := followUpCmd(); m != nil {
			t.Fatalf("已调度过同一序号请求后不应重复触发，收到 %T", m)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("二维码请求次数 = %d, want 1", got)
	}
}

// TestQQUserPlaylistHookTriggersQRGenerationWithoutTabSwitch 复现从主菜单进入
// 「我的歌单」触发登录页的真实链路：BeforeEnterMenuHook 只能返回 model.Page 而
// 丢弃了 ToQQLoginPage 返回的 tea.Cmd，随后框架仅向新页面发送一次 page.Msg()。
func TestQQUserPlaylistHookTriggersQRGenerationWithoutTabSwitch(t *testing.T) {
	app, n, _ := setupSwitchSourceTestApp(t)
	n.SetSource(SourceQQ)

	playlistMenu := NewQQUserPlaylistMenu(newBaseMenu(n))
	ok, nextPage := playlistMenu.BeforeEnterMenuHook()(app.MustMain())
	if ok {
		t.Fatal("未登录时进入我的歌单应被拦截并跳转到登录页")
	}
	qrPage, isQR := nextPage.(*QQLoginPage)
	if !isQR {
		t.Fatalf("跳转目标不是 *QQLoginPage: %T", nextPage)
	}

	var calls atomic.Int32
	pngBytes := buildTestQRPNG(t, "https://example.com/qq-hook")
	qrPage.fetchQR = func(loginType qqmusic.QRLoginType) (*qqmusic.QRCode, error) {
		calls.Add(1)
		return &qqmusic.QRCode{
			Image:      pngBytes,
			MimeType:   "image/png",
			Identifier: "hook-qrsig",
			Type:       loginType,
		}, nil
	}

	// 模拟 foxful-cli 在 Hook 返回 (false, newPage) 后投递 newPage.Msg()
	_, wakeCmd := qrPage.Update(qrPage.Msg(), app)
	if wakeCmd == nil {
		t.Fatal("收到首次唤醒消息后必须补发二维码生成 cmd，否则会卡在「正在获取二维码...」")
	}

	genMsg := findQRGeneratedMsg(wakeCmd())
	if genMsg == nil {
		t.Fatal("唤醒 cmd 未产出 qqQRGeneratedMsg")
	}

	// 请求进行中或完成后再次收到帧率 tick，不应重复拉取二维码
	_, dupCmd := qrPage.Update(qrPage.Msg(), app)
	if dupCmd != nil {
		if m := dupCmd(); m != nil {
			t.Fatalf("后续 tickLoginMsg 不应重复发起请求，收到 %T", m)
		}
	}

	_, _ = qrPage.Update(*genMsg, app)
	if qrPage.qrView == "" {
		t.Error("处理 qqQRGeneratedMsg 后 qrView 不应为空")
	}
	if qrPage.statusMsg != "请使用手机 QQ 扫描二维码" {
		t.Errorf("statusMsg = %q, want \"请使用手机 QQ 扫描二维码\"", qrPage.statusMsg)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("二维码请求次数 = %d, want 1", got)
	}
}

// TestQQLoginPageDropsStaleTabResponseAndSupportsRetryOnError 校验：
//  1. 快速切换 QQ/微信 Tab 时，延迟返回的旧 Tab 二维码不会覆盖当前 Tab；
//  2. 获取二维码报错后，按 r 键可以直接重试。
func TestQQLoginPageDropsStaleTabResponseAndSupportsRetryOnError(t *testing.T) {
	app, n, _ := setupSwitchSourceTestApp(t)
	n.SetSource(SourceQQ)

	page, firstCmd := n.ToQQLoginPage(nil)
	qrPage := page.(*QQLoginPage)

	pngQQ := buildTestQRPNG(t, "https://example.com/qq-1")
	pngWX := buildTestQRPNG(t, "https://example.com/wx-2")

	// 在首个 QQ 二维码请求返回前，用户切换到微信扫码 Tab
	qrPage.fetchQR = func(loginType qqmusic.QRLoginType) (*qqmusic.QRCode, error) {
		if loginType == qqmusic.QRLoginWX {
			return nil, errors.New("network timeout")
		}
		return &qqmusic.QRCode{Image: pngQQ, MimeType: "image/png", Identifier: "qq-stale", Type: loginType}, nil
	}
	_, wxCmd := qrPage.switchTab()

	// 旧的 QQ 请求晚一步返回，应被序号过滤丢弃
	staleMsg := findQRGeneratedMsg(firstCmd())
	if staleMsg == nil {
		t.Fatal("firstCmd 未产出消息")
	}
	_, _ = qrPage.Update(*staleMsg, app)
	if qrPage.qr != nil {
		t.Fatal("过期的旧 Tab 二维码响应不应写入当前页面")
	}

	// 微信 Tab 首次请求返回网络错误，应允许按 r 重试
	errMsg := findQRGeneratedMsg(wxCmd())
	if errMsg == nil || errMsg.err == nil {
		t.Fatalf("期望微信首次请求返回错误，实际: %+v", errMsg)
	}
	_, _ = qrPage.Update(*errMsg, app)
	if !qrPage.expired {
		t.Fatal("获取二维码失败后应标记 expired=true 以允许按 r 重试")
	}
	if !strings.Contains(qrPage.statusMsg, "获取二维码失败") {
		t.Errorf("错误提示缺失: %q", qrPage.statusMsg)
	}

	// 恢复网络后按 r 重试
	qrPage.fetchQR = func(loginType qqmusic.QRLoginType) (*qqmusic.QRCode, error) {
		return &qqmusic.QRCode{Image: pngWX, MimeType: "image/png", Identifier: "wx-ok", Type: loginType}, nil
	}
	_, retryCmd := qrPage.Update(tea.KeyPressMsg{Code: 'r', Text: "r"}, app)
	if retryCmd == nil {
		t.Fatal("获取失败后按 r 应触发重新获取二维码")
	}
	okMsg := findQRGeneratedMsg(retryCmd())
	if okMsg == nil || okMsg.err != nil {
		t.Fatalf("重试未成功生成二维码: %+v", okMsg)
	}
	_, _ = qrPage.Update(*okMsg, app)
	if qrPage.expired || qrPage.qr == nil || qrPage.qr.Identifier != "wx-ok" {
		t.Fatalf("重试成功后状态未恢复: expired=%v qr=%+v", qrPage.expired, qrPage.qr)
	}
	if qrPage.statusMsg != "请使用微信扫描二维码" {
		t.Errorf("statusMsg = %q, want \"请使用微信扫描二维码\"", qrPage.statusMsg)
	}
	qrPage.cleanupQRFile()
}

// findQRGeneratedMsg 递归查找 BatchMsg 中的 qqQRGeneratedMsg。
func findQRGeneratedMsg(msg any) *qqQRGeneratedMsg {
	if m, ok := msg.(qqQRGeneratedMsg); ok {
		return &m
	}
	// tea.Batch 产出的 BatchMsg 是 []tea.Cmd，需逐个执行子命令。
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			if got := findQRGeneratedMsg(sub()); got != nil {
				return got
			}
		}
	}
	return nil
}

// TestQQLoginPageInitSignature 校验 Init 符合 model.InitPage 接口。
//
// foxful-cli 要求 `Init(a *App) tea.Cmd`。若签名写成无参版本，
// 类型断言 `a.page.(InitPage)` 会失败，框架永不调用。
func TestQQLoginPageInitSignature(t *testing.T) {
	app, n, _ := setupSwitchSourceTestApp(t)
	n.SetSource(SourceQQ)

	page := NewQQLoginPage(n, app.MustMain(), nil)

	// 编译期断言：必须实现 model.InitPage（含带 *model.App 的 Init 方法）
	var _ model.InitPage = page

	// 运行期确认能正常调用并返回 cmd
	cmd := page.Init(app)
	if cmd == nil {
		t.Error("Init 应返回生成二维码的 cmd")
	}
}

// TestSwitchTabStillWorks 校验切换 Tab 仍能重新生成二维码。
func TestSwitchTabStillWorks(t *testing.T) {
	app, n, _ := setupSwitchSourceTestApp(t)
	n.SetSource(SourceQQ)

	page := NewQQLoginPage(n, app.MustMain(), nil)
	initialTab := page.activeTab

	newPage, cmd := page.switchTab()
	switched, ok := newPage.(*QQLoginPage)
	if !ok {
		t.Fatalf("switchTab 返回类型错误: %T", newPage)
	}

	if switched.activeTab == initialTab {
		t.Error("activeTab 应已切换")
	}
	if switched.qrView != "" {
		t.Error("切换后 qrView 应清空，等待重新生成")
	}
	if switched.statusMsg != "正在获取二维码..." {
		t.Errorf("切换后 statusMsg = %q, want \"正在获取二维码...\"", switched.statusMsg)
	}
	if cmd == nil {
		t.Error("switchTab 应返回生成二维码的 cmd")
	}
}
