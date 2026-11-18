package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// TestQQLoginPageQRCodeFitsHeight 是二维码尺寸的回归测试。
//
// 历史 bug：二维码按像素渲染得到 111 列 × 56 行，远超终端，用户看不到完整二维码。
// 修复后为 33 列 × 17 行，且页面留白会随终端高度自适应压缩。
func TestQQLoginPageQRCodeFitsHeight(t *testing.T) {
	// 33 模块 → 33 列、17 行，模拟渲染结果。
	qr := buildFakeQRView(33, 17)

	sizes := []struct {
		name          string
		width, height int
		// wantQR 为 false 表示该尺寸放不下二维码，页面应给出降级提示。
		wantQR bool
	}{
		{"标准 80x24", 80, 24, true},
		{"紧凑 80x20", 80, 20, true},
		{"宽屏 120x40", 120, 40, true},
		// 18 行放不下「标题 + 17 行二维码」，应提示用图片查看器。
		{"过矮 60x18", 60, 18, false},
	}

	for _, tc := range sizes {
		t.Run(tc.name, func(t *testing.T) {
			app, n, _ := setupSwitchSourceTestApp(t)
			_, _ = app.Update(tea.WindowSizeMsg{Width: tc.width, Height: tc.height})

			page := NewQQLoginPage(n, app.MustMain(), nil)
			page.qrView = qr
			page.statusMsg = "请使用手机 QQ 扫描二维码"

			view := page.View(app)
			lines := strings.Split(view, "\n")

			if !tc.wantQR {
				// 高度不足时应出现降级提示，而不是残缺二维码。
				if !strings.Contains(view, "终端高度不足") {
					t.Error("高度不足时缺少降级提示")
				}
				if lipgloss.Height(view) > tc.height {
					t.Errorf("渲染高度 %d 超过终端高度 %d", lipgloss.Height(view), tc.height)
				}
				return
			}

			// 二维码的 17 行必须全部出现在渲染结果里。
			qrRows := strings.Split(strings.TrimSuffix(qr, "\n"), "\n")
			present := 0
			for _, qrLine := range qrRows {
				trimmed := strings.TrimRight(qrLine, " ")
				if trimmed == "" {
					present++
					continue
				}
				for _, line := range lines {
					if strings.Contains(line, trimmed) {
						present++
						break
					}
				}
			}
			if present != len(qrRows) {
				t.Errorf("二维码仅显示 %d/%d 行，未完整显示", present, len(qrRows))
			}

			// 渲染高度不应超过终端高度（否则会被截断）。
			gotHeight := lipgloss.Height(view)
			if gotHeight > tc.height {
				t.Errorf("渲染高度 %d 超过终端高度 %d", gotHeight, tc.height)
			}

			// 每行可见宽度（不含 ANSI 转义码）不应超过终端宽度。
			for i, line := range lines {
				if w := lipgloss.Width(line); w > tc.width {
					t.Errorf("第 %d 行可见宽度 %d 超过终端宽度 %d", i, w, tc.width)
				}
			}
		})
	}
}

// TestQQLoginPageVerticalGapShrinks 校验留白随高度压缩。
func TestQQLoginPageVerticalGapShrinks(t *testing.T) {
	page := &QQLoginPage{}

	// 高终端：留白取默认值 1
	if got := page.verticalGap(40, 3, 17); got != 1 {
		t.Errorf("40 行终端留白 = %d, want 1", got)
	}
	// 24 行终端：3 + 17 + 3 = 23，剩 1，留白 0
	if got := page.verticalGap(24, 3, 17); got != 0 {
		t.Errorf("24 行终端留白 = %d, want 0", got)
	}
	// 极矮终端不应产生负留白
	if got := page.verticalGap(10, 3, 17); got != 0 {
		t.Errorf("10 行终端留白 = %d, want 0（不可为负）", got)
	}
}

// buildFakeQRView 构造一个 size×size 模块的半块字符画，模拟真实渲染结果。
func buildFakeQRView(size, rows int) string {
	var sb strings.Builder
	for y := 0; y < rows; y++ {
		for x := 0; x < size; x++ {
			// 用可区分的图案，便于在输出中定位
			switch {
			case (x+y)%3 == 0:
				sb.WriteString("█")
			case (x+y)%3 == 1:
				sb.WriteString("▀")
			default:
				sb.WriteString("▄")
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
