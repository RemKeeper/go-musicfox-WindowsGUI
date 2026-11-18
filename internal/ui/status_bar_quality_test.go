package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestStatusBarShowsActualQuality 校验状态栏显示实际音质而非配置值。
//
// 历史 bug：状态栏直接读 configs.AppConfig.Player.SongLevel，
// 切换歌曲或降级后仍显示配置里的期望音质，与实际播放不符。
func TestStatusBarShowsActualQuality(t *testing.T) {
	// 直接构造最小 Player（只需音质字段，不启动播放器）
	player := &Player{}

	// 未播放时：CurQuality 为空，状态栏应回退到配置值
	if got := player.CurQuality(); got != "" {
		t.Errorf("初始 CurQuality = %q, want 空串", got)
	}

	// 模拟解析出音源后记录实际音质
	player.setCurQuality("无损", 0)
	if got := player.CurQuality(); got != "无损" {
		t.Errorf("CurQuality = %q, want \"无损\"", got)
	}

	// 再切到另一个音质
	player.setCurQuality("HQ", 0)
	if got := player.CurQuality(); got != "HQ" {
		t.Errorf("CurQuality = %q, want \"HQ\"", got)
	}
}

// TestCurQualityConcurrentSafe 校验 CurQuality 并发读写安全。
func TestCurQualityConcurrentSafe(t *testing.T) {
	player := &Player{}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			player.setCurQuality("无损", 0)
			player.setCurQuality("", 0)
		}
	}()

	for i := 0; i < 200; i++ {
		_ = player.CurQuality()
	}
	<-done
}

// TestSourceNameFollowsSwitch 校验顶栏标题随渠道切换更新。
//
// 历史 bug：标题硬编码为「网易云音乐」，切到 QQ 音乐后顶栏不变。
func TestSourceNameFollowsSwitch(t *testing.T) {
	app, n, _ := setupSwitchSourceTestApp(t)

	// 初始为网易云
	if got := n.SourceName(); got != "网易云音乐" {
		t.Errorf("初始 SourceName = %q, want \"网易云音乐\"", got)
	}

	// 模拟真实启动流程：标题由 WithMainMenu 设置
	app.MustMain().MenuTitle().Title = n.SourceName()
	app.MustMain().RefreshMenuTitle()
	if title := app.MustMain().MenuTitle(); title == nil || title.Title != "网易云音乐" {
		t.Errorf("初始顶栏标题 = %v, want \"网易云音乐\"", title)
	}

	// 切到 QQ 音乐
	n.SetSource(SourceQQ)
	if got := n.SourceName(); got != "QQ音乐" {
		t.Errorf("切换后 SourceName = %q, want \"QQ音乐\"", got)
	}
	if title := app.MustMain().MenuTitle(); title == nil || title.Title != "QQ音乐" {
		t.Errorf("切换后顶栏标题 = %v, want \"QQ音乐\"", title)
	}

	// 切回网易云
	n.SetSource(SourceNetease)
	if title := app.MustMain().MenuTitle(); title == nil || title.Title != "网易云音乐" {
		t.Errorf("切回后顶栏标题 = %v, want \"网易云音乐\"", title)
	}
}

// TestSourceNameInRenderedView 校验渲染出的视图里确实出现渠道名。
func TestSourceNameInRenderedView(t *testing.T) {
	app, n, _ := setupSwitchSourceTestApp(t)

	n.SetSource(SourceQQ)
	_, _ = app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	view := app.MustMain().View(app)

	if !strings.Contains(view, "QQ音乐") {
		t.Error("渲染视图里未出现「QQ音乐」")
	}
	if strings.Contains(view, "网易云音乐") {
		t.Error("切到 QQ 音乐后视图里仍出现「网易云音乐」")
	}
}
