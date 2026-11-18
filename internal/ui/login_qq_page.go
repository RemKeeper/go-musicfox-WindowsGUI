package ui

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/style"
	"github.com/anhoder/foxful-cli/util"
	"github.com/mattn/go-runewidth"
	"github.com/skratchdot/open-golang/open"

	"github.com/go-musicfox/go-musicfox/internal/qqmusic"
	"github.com/go-musicfox/go-musicfox/internal/structs"
	"github.com/go-musicfox/go-musicfox/utils/app"
	"github.com/go-musicfox/go-musicfox/utils/errorx"
	"github.com/go-musicfox/go-musicfox/utils/slogx"
)

const QQLoginPageType model.PageType = "qq_login"

// qqMaxPollErrors 是连续轮询失败的上限，超过后提示用户刷新二维码。
//
// 轮询失败通常意味着 qrsig 失效或网络异常，无限重试会让界面永远停在
// 「等待扫码」，用户无法判断是没扫码还是程序出错。
const qqMaxPollErrors = 5

// QQ 登录页的焦点索引。
const (
	qqLoginTabQQ = iota
	qqLoginTabWX
	qqLoginTabCount
)

// qqQRGeneratedMsg 是二维码生成完成消息。
type qqQRGeneratedMsg struct {
	seq    uint64
	qr     *qqmusic.QRCode
	qrView string
	qrPath string
	err    error
}

// qqQRStatusMsg 是一次轮询结果。
type qqQRStatusMsg struct {
	result qqmusic.QRLoginResult
}

// qqTickPollingMsg 触发下一次轮询。
type qqTickPollingMsg struct{}

func qqTickPolling(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return qqTickPollingMsg{} })
}

// QQLoginPage 是 QQ 音乐扫码登录页。
//
// 支持 QQ 与微信两种扫码方式，通过 Tab 切换。登录成功后复用网易云登录页
// 的回调机制（LoginCallback），使「登录后继续原操作」的体验保持一致。
type QQLoginPage struct {
	netease *Netease
	from    model.Page

	activeTab int
	qr        *qqmusic.QRCode
	qrView    string
	qrPath    string
	expired   bool
	statusMsg string
	loading   *model.Loading
	// pollErrors 是连续轮询失败次数，达到 qqMaxPollErrors 后提示用户刷新。
	pollErrors int

	// qrReqSeq is the current expected QR request sequence number, incremented on
	// each new request (initial load, tab switch, manual refresh) to drop stale responses.
	qrReqSeq uint64
	// qrDispatchedSeq tracks the latest sequence number actually executed by BubbleTea.
	// Callers like BeforeEnterMenuHook cannot return a tea.Cmd and discard the second
	// return value of ToQQLoginPage; when Update receives the initial tickLoginMsg wake-up,
	// qrDispatchedSeq < qrReqSeq signals that the command still needs to be scheduled.
	qrDispatchedSeq atomic.Uint64
	// fetchQR overrides QR fetching in tests when non-nil.
	fetchQR func(qqmusic.QRLoginType) (*qqmusic.QRCode, error)

	AfterLogin LoginCallback

	backBtnHovered bool
	backBtnRowY    int
	backBtnStartX  int
	backBtnEndX    int
	mousePointer   string
}

// NewQQLoginPage 创建 QQ 登录页。
func NewQQLoginPage(netease *Netease, from model.Page, afterLogin LoginCallback) *QQLoginPage {
	page := &QQLoginPage{
		netease:    netease,
		from:       from,
		AfterLogin: afterLogin,
		statusMsg:  "正在获取二维码...",
	}
	page.loading = model.NewLoading(netease.MustMain(), &model.MenuItem{Title: "QQ音乐登录"})
	page.loading.DisplayNotOnlyOnMain()
	return page
}

// ToQQLoginPage 跳转到 QQ 登录页。
//
// 同时兼容两种跳转方式：
//  1. 调用方保留并执行返回的 tea.Cmd；
//  2. 调用方（如 BeforeEnterMenuHook、likeQQSong）只能返回 model.Page 而丢弃 tea.Cmd，
//     此时框架切页后发送 page.Msg()（tickLoginMsg），由 Update 检测未调度的序号并自动补发。
func (n *Netease) ToQQLoginPage(callback func() model.Page) (model.Page, tea.Cmd) {
	page := NewQQLoginPage(n, n.MustMain(), callback)
	if n.coverRenderer != nil {
		n.coverRenderer.ClearDisplayed()
	}
	cmd := page.startQRGeneration()
	return page, tea.Batch(qqTickPolling(time.Nanosecond), cmd)
}

func (p *QQLoginPage) IgnoreQuitKeyMsg(_ tea.KeyMsg) bool { return true }

func (p *QQLoginPage) Msg() tea.Msg { return tickLoginMsg{} }

func (p *QQLoginPage) Type() model.PageType { return QQLoginPageType }

// Init 实现 model.InitPage。
func (p *QQLoginPage) Init(_ *model.App) tea.Cmd {
	if p.qrReqSeq == 0 {
		return p.startQRGeneration()
	}
	return p.generateQRCodeCmd(p.qrReqSeq)
}

func (p *QQLoginPage) Update(msg tea.Msg, a *model.App) (model.Page, tea.Cmd) {
	switch msg := msg.(type) {
	case tickLoginMsg:
		if p.qrReqSeq == 0 {
			return p, p.startQRGeneration()
		}
		if p.qrDispatchedSeq.Load() < p.qrReqSeq {
			return p, p.generateQRCodeCmd(p.qrReqSeq)
		}
		return p, nil

	case tea.MouseMotionMsg:
		mouse := msg.Mouse()
		oldHovered := p.backBtnHovered
		oldPointer := p.mousePointer
		bcChanged, bcOver := pageBreadcrumbMotion(a, p.netease.MustMain(), mouse.X, mouse.Y)
		p.backBtnHovered = mouse.Y == p.backBtnRowY && mouse.X >= p.backBtnStartX && mouse.X < p.backBtnEndX
		p.mousePointer = "default"
		if p.backBtnHovered || bcOver {
			p.mousePointer = "pointer"
		}
		if p.backBtnHovered != oldHovered || p.mousePointer != oldPointer || bcChanged {
			return p, tea.Sequence(p.netease.RerenderCmd(true), a.SetMousePointer(p.mousePointer))
		}
		return p, nil

	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		if mouse.Button == tea.MouseLeft {
			if newPage := pageBreadcrumbClick(a, p.netease.MustMain(), mouse.X, mouse.Y); newPage != nil {
				return newPage, p.netease.RerenderCmd(true)
			}
			if mouse.Y == p.backBtnRowY && mouse.X >= p.backBtnStartX && mouse.X < p.backBtnEndX {
				return p.back()
			}
		}
		return p, nil

	case qqQRGeneratedMsg:
		if msg.seq != p.qrReqSeq {
			if msg.qrPath != "" && msg.qrPath != p.qrPath {
				_ = os.Remove(msg.qrPath)
			}
			return p, nil
		}
		p.loading.Complete()
		if msg.err != nil {
			p.expired = true
			p.statusMsg = util.SetFgStyle("获取二维码失败: "+msg.err.Error()+"（按 r 重试）", lipgloss.BrightRed)
			return p, nil
		}
		p.qr = msg.qr
		p.qrView = msg.qrView
		p.qrPath = msg.qrPath
		p.expired = false
		if p.activeTab == qqLoginTabWX {
			p.statusMsg = "请使用微信扫描二维码"
		} else {
			p.statusMsg = "请使用手机 QQ 扫描二维码"
		}
		return p, qqTickPolling(1500 * time.Millisecond)

	case qqTickPollingMsg:
		if p.qr == nil || p.expired {
			return p, nil
		}
		return p, p.pollQRStatus

	case qqQRStatusMsg:
		if msg.result.Err != nil {
			// 轮询失败不中断流程，但必须让用户看见，否则会一直显示
			// 「等待扫码」而实际请求全部失败（历史上 ptqrtoken 算错导致 403 就是这样）。
			p.pollErrors++
			slog.Warn("QQ 扫码轮询失败", "count", p.pollErrors, slogx.Error(msg.result.Err))

			if p.pollErrors >= qqMaxPollErrors {
				p.statusMsg = util.SetFgStyle(
					"轮询失败，请按 r 刷新二维码重试", lipgloss.BrightRed)
				p.expired = true
				p.cleanupQRFile()
				return p, nil
			}
			p.statusMsg = util.SetFgStyle(
				fmt.Sprintf("网络异常，重试中(%d/%d)...", p.pollErrors, qqMaxPollErrors),
				lipgloss.BrightYellow)
			return p, qqTickPolling(2 * time.Second)
		}
		// 成功一次就清零，避免偶发抖动累积到阈值。
		p.pollErrors = 0

		switch msg.result.Event {
		case qqmusic.QREventWaiting:
			p.statusMsg = "等待扫码..."
			return p, qqTickPolling(1500 * time.Millisecond)
		case qqmusic.QREventScanned:
			p.statusMsg = "已扫码，请在手机上确认"
			return p, qqTickPolling(1200 * time.Millisecond)
		case qqmusic.QREventConfirmed:
			p.statusMsg = "已确认，正在登录..."
			return p, qqTickPolling(800 * time.Millisecond)
		case qqmusic.QREventRefused:
			p.statusMsg = util.SetFgStyle("登录已被拒绝", lipgloss.BrightRed)
			p.expired = true
			return p, nil
		case qqmusic.QREventExpired:
			p.statusMsg = util.SetFgStyle("二维码已过期，按 r 重新获取", lipgloss.BrightRed)
			p.expired = true
			p.cleanupQRFile()
			return p, nil
		case qqmusic.QREventDone:
			return p.onLoginSuccess(msg.result.Credential)
		}
		return p, qqTickPolling(1500 * time.Millisecond)

	case tea.KeyPressMsg:
		switch msg.String() {
		case "b", "esc", "q":
			return p.back()
		case "r":
			if p.expired {
				p.expired = false
				p.statusMsg = "正在重新获取二维码..."
				return p, p.startQRGeneration()
			}
		case "tab", "right", "l":
			return p.switchTab()
		case "left", "h":
			return p.switchTab()
		case "v":
			if p.qrPath != "" && !p.expired {
				if err := open.Start(p.qrPath); err != nil {
					p.statusMsg = util.SetFgStyle("打开二维码图片失败: "+err.Error(), lipgloss.BrightRed)
				}
			}
		}
	}

	return p, nil
}

// switchTab 在 QQ / 微信扫码之间切换。
func (p *QQLoginPage) switchTab() (model.Page, tea.Cmd) {
	p.activeTab = (p.activeTab + 1) % qqLoginTabCount
	p.cleanupQRFile()
	p.qr = nil
	p.qrView = ""
	p.expired = false
	p.statusMsg = "正在获取二维码..."
	return p, p.startQRGeneration()
}

// onLoginSuccess 处理登录成功。
func (p *QQLoginPage) onLoginSuccess(cred *qqmusic.Credential) (model.Page, tea.Cmd) {
	p.cleanupQRFile()
	if cred == nil {
		p.statusMsg = util.SetFgStyle("登录响应异常：凭证为空", lipgloss.BrightRed)
		return p, nil
	}

	p.netease.qqClient.SetCredential(*cred)
	p.netease.qqUser = &structs.User{
		UserId:   cred.MusicID,
		Nickname: fmt.Sprintf("QQ用户%d", cred.MusicID),
	}
	p.statusMsg = "登录成功"

	// 持久化凭证，下次启动免登录。
	if err := saveQQCredential(*cred); err != nil {
		slog.Warn("QQ 音乐凭证持久化失败", slogx.Error(err))
	}

	// 后台同步「我喜欢」，使爱心图标在登录后立即可用。
	errorx.Go(func() { refreshQQLikeList(p.netease) }, false)

	p.netease.NotifyInfo("QQ 音乐登录成功", p.netease.qqUser.Nickname)

	if p.AfterLogin != nil {
		if next := p.AfterLogin(); next != nil {
			return next, p.netease.RerenderCmd(true)
		}
	}
	return p.netease.MustMain(), tea.Batch(tea.ClearScreen, model.TickMain(time.Nanosecond), p.netease.RerenderCmd(true))
}

// back 返回来源页。
func (p *QQLoginPage) back() (model.Page, tea.Cmd) {
	p.cleanupQRFile()
	return p.from, p.netease.RerenderCmd(true)
}

// cleanupQRFile 删除临时二维码图片。
func (p *QQLoginPage) cleanupQRFile() {
	if p.qrPath != "" {
		_ = os.Remove(p.qrPath)
		p.qrPath = ""
	}
}

// startQRGeneration allocates a new request sequence, resets polling error state,
// starts the loading indicator, and returns a single-flight tea.Cmd for that sequence.
func (p *QQLoginPage) startQRGeneration() tea.Cmd {
	p.qrReqSeq++
	p.pollErrors = 0
	p.loading.Start()
	return p.generateQRCodeCmd(p.qrReqSeq)
}

// generateQRCodeCmd binds loginType/filename on the main goroutine and ensures
// each request sequence is dispatched at most once across ToQQLoginPage and tickLoginMsg.
func (p *QQLoginPage) generateQRCodeCmd(seq uint64) tea.Cmd {
	loginType := qqmusic.QRLoginQQ
	name := "qq-qrcode.png"
	if p.activeTab == qqLoginTabWX {
		loginType = qqmusic.QRLoginWX
		name = "wx-qrcode.png"
	}
	return func() tea.Msg {
		for {
			cur := p.qrDispatchedSeq.Load()
			if cur >= seq {
				return nil
			}
			if p.qrDispatchedSeq.CompareAndSwap(cur, seq) {
				break
			}
		}
		return p.generateQRCode(seq, loginType, name)
	}
}

// generateQRCode 异步获取二维码并渲染到终端。
func (p *QQLoginPage) generateQRCode(seq uint64, loginType qqmusic.QRLoginType, name string) tea.Msg {
	var (
		qr  *qqmusic.QRCode
		err error
	)
	if p.fetchQR != nil {
		qr, err = p.fetchQR(loginType)
	} else {
		if p.netease.qqClient == nil {
			return qqQRGeneratedMsg{seq: seq, err: fmt.Errorf("QQ 音乐渠道未初始化")}
		}
		qr, err = p.netease.qqClient.GetQRCode(loginType)
	}
	if err != nil {
		return qqQRGeneratedMsg{seq: seq, err: err}
	}

	// 二维码图片需要落盘后再渲染：GenQRCodeFromImage 把图片字节写成文件，
	// 并渲染成终端半块字符画。
	path, view, err := app.GenQRCodeFromImage(name, qr.Image)
	if err != nil {
		return qqQRGeneratedMsg{seq: seq, err: err}
	}

	return qqQRGeneratedMsg{seq: seq, qr: qr, qrView: view.String(), qrPath: path}
}

// pollQRStatus 轮询一次扫码状态。
func (p *QQLoginPage) pollQRStatus() tea.Msg {
	if p.qr == nil {
		return qqQRStatusMsg{result: qqmusic.QRLoginResult{Event: qqmusic.QREventWaiting}}
	}
	return qqQRStatusMsg{result: p.netease.qqClient.CheckQRCode(p.qr)}
}

func (p *QQLoginPage) View(a *model.App) string {
	var builder strings.Builder

	var top int
	mainPage := p.netease.MustMain()
	builder.WriteString(pageTitleView(a, mainPage, &top))
	topBefore := top
	// 二维码页需要尽可能多的高度，因此不复用 pageMenuTitleViewWithBack：
	// 后者会把标题对齐到主菜单标题行（MenuStartRow-3），在窄终端上会浪费
	// 数行空白，导致二维码放不下。这里紧凑地从顶部渲染标题。
	startColumn := max(0, mainPage.MenuStartColumn())
	padding := max(0, startColumn-pageBackButtonWidth)
	builder.WriteString(style.CurrentStyleSet().AppBackground.Render(strings.Repeat(" ", padding)))
	builder.WriteString(pageBackButtonIcon(p.backBtnHovered))
	builder.WriteString(" ")
	builder.WriteString(style.CurrentStyleSet().MenuTitle.Render("QQ音乐登录"))
	builder.WriteString("\n")
	top++

	p.backBtnRowY = topBefore
	p.backBtnStartX = max(0, startColumn-pageBackButtonWidth)
	p.backBtnEndX = p.backBtnStartX + pageBackButtonWidth

	qrLines := []string{}
	if p.qrView != "" {
		qrLines = strings.Split(strings.TrimSuffix(p.qrView, "\n"), "\n")
	}

	// 终端高度不足以容纳「二维码 + Tab + 状态 + 提示」时，二维码必然被截断，
	// 此时改为提示用图片查看器打开，避免显示一个扫不出来的残缺二维码。
	qrRows := len(qrLines)
	sep := p.verticalGap(a.WindowHeight(), top, qrRows)
	canFitQR := qrRows > 0 && a.WindowHeight() >= top+qrRows

	builder.WriteString(strings.Repeat("\n", sep))
	top += sep

	// Tab 行
	tabs := []string{"QQ扫码", "微信扫码"}
	var tabLine strings.Builder
	for i, name := range tabs {
		if i == p.activeTab {
			tabLine.WriteString(pageButton(name, true))
		} else {
			tabLine.WriteString(pageButton(name, false))
		}
		tabLine.WriteString("  ")
	}
	builder.WriteString(centerLine(tabLine.String(), a.WindowWidth()))
	builder.WriteString(strings.Repeat("\n", sep))

	switch {
	case qrRows == 0:
		// 二维码尚未生成。
		builder.WriteString(strings.Repeat("\n", 8))
	case !canFitQR:
		builder.WriteString(centerLine(
			util.SetFgStyle("终端高度不足，无法完整显示二维码", lipgloss.BrightRed),
			a.WindowWidth()))
		builder.WriteString(centerLine(
			util.SetFgStyle("请按 v 用图片查看器打开，或放大终端窗口", lipgloss.BrightBlack),
			a.WindowWidth()))
	default:
		qrWidth := runewidth.StringWidth(qrLines[0])
		// 终端过窄时二维码会被截断而无法扫描，此时提示改用图片查看。
		if qrWidth > a.WindowWidth() {
			builder.WriteString(centerLine(
				util.SetFgStyle("终端宽度不足，无法完整显示二维码，请按 v 用图片查看器打开", lipgloss.BrightRed),
				a.WindowWidth()))
		}

		padding := (a.WindowWidth() - qrWidth) / 2
		if padding < 0 {
			padding = 0
		}
		space := style.CurrentStyleSet().AppBackground.Render(strings.Repeat(" ", padding))

		if p.expired {
			expiredMsg := "二维码已过期"
			msgWidth := runewidth.StringWidth(expiredMsg)
			msgPadLen := (qrWidth - msgWidth) / 2
			if msgPadLen < 0 {
				msgPadLen = 0
			}
			fullMsg := strings.Repeat(" ", msgPadLen) + expiredMsg
			for runewidth.StringWidth(fullMsg) < qrWidth {
				fullMsg += " "
			}
			if mid := len(qrLines) / 2; mid > 0 && mid < len(qrLines) {
				qrLines[mid] = fullMsg
			}
		}

		for _, line := range qrLines {
			builder.WriteString(space)
			if p.expired {
				builder.WriteString(util.SetFgStyle(line, lipgloss.BrightRed))
			} else {
				builder.WriteString(line)
			}
			builder.WriteString("\n")
		}
	}

	builder.WriteString(centerLine(p.statusMsg, a.WindowWidth()))
	builder.WriteString(centerLine(util.SetFgStyle(
		"Tab 切换扫码方式  ·  r 刷新  ·  v 查看图片  ·  b/esc 返回", lipgloss.BrightBlack), a.WindowWidth()))

	// 终端高度紧张时，宁可丢掉末尾的提示行，也不能让二维码被截断：
	// 截断的二维码无法扫描，而提示行只是辅助信息。
	view := builder.String()
	if qrRows > 0 {
		view = clampPageHeight(view, a.WindowHeight())
	}
	fillPageHeight(&builder, a.WindowHeight())
	if qrRows > 0 {
		return model.RenderAppBackground(view, a.WindowWidth())
	}
	return finishCustomPageView(&builder, a)
}

// clampPageHeight 在内容超出终端高度时，从末尾丢弃多余行。
//
// 二维码页的二维码必须完整显示，因此超出的部分只能从尾部（提示行）裁掉。
// 末尾换行会让 lipgloss.Height 多算一行，因此按 height-1 行内容裁剪后再补换行。
func clampPageHeight(view string, height int) string {
	if height <= 1 {
		return view
	}
	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	if len(lines) <= height-1 {
		return view
	}
	return strings.Join(lines[:height-1], "\n") + "\n"
}

// verticalGap 计算二维码上下分隔空行数。
//
// 二维码行数不可压缩（压缩后无法扫描），因此当终端高度紧张时优先压缩留白。
// usedRows 是标题区实际已占用的行数，qrRows 是二维码行数；
// Tab、状态、提示各占 1 行。
func (p *QQLoginPage) verticalGap(windowHeight, usedRows, qrRows int) int {
	const (
		// Tab + 状态 + 提示
		chromeRows = 3
		// 默认上下各留 1 行空白，视觉上不拥挤。
		defaultGap = 1
	)

	remaining := windowHeight - usedRows - qrRows - chromeRows
	gap := remaining / 2
	if gap > defaultGap {
		gap = defaultGap
	}
	if gap < 0 {
		gap = 0
	}
	return gap
}

// centerLine 将一行文本按窗口宽度居中。
func centerLine(s string, width int) string {
	padding := (width - runewidth.StringWidth(s)) / 2
	if padding < 0 {
		padding = 0
	}
	return style.CurrentStyleSet().AppBackground.Render(strings.Repeat(" ", padding)) + s + "\n"
}
