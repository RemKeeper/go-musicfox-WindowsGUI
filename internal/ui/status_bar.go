package ui

import (
	"fmt"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/style"
	"github.com/anhoder/foxful-cli/util"
	"github.com/go-musicfox/netease-music/service"
	"github.com/skratchdot/open-golang/open"

	"github.com/go-musicfox/go-musicfox/internal/configs"
	"github.com/go-musicfox/go-musicfox/internal/types"
)

const musicfoxStatusBarLabel = "musicfox"

// qualityDisplayName 返回音质的中文显示名称。
func qualityDisplayName(level service.SongQualityLevel) string {
	switch level {
	case service.Standard:
		return "标准"
	case service.Higher:
		return "较高"
	case service.Exhigh:
		return "极高"
	case service.Lossless:
		return "无损"
	case service.Hires:
		return "Hi-Res"
	case service.JYEffect:
		return "高清环绕"
	case service.Sky:
		return "沉浸环绕"
	case service.JYMaster:
		return "超清母带"
	default:
		return string(level)
	}
}

// formatQueueAndQuality 格式化状态栏中间文本。
// 格式：musicfox · [当前索引/总数] · 音质 [· 码率 · 大小]
// 若无播放歌曲或播放列表为空，返回空字符串。
//
// 音质优先显示「当前歌曲实际拿到的音质」（降级链可能回退），
// 未解析出音源时才回退到配置里的期望值。
//
// 码率与大小仅在可计算时显示（需要文件大小与歌曲时长），
// 典型的 QQ 渠道输出：musicfox · [2/100] · SQ · 549 kbps · 14.4 MB
func formatQueueAndQuality(player *Player) string {
	song := player.CurSong()
	if song.Id == 0 {
		return ""
	}

	playlist := player.Playlist()
	if len(playlist) == 0 {
		return ""
	}

	// 队列位置（1-indexed 显示）
	curIndex := player.CurSongIndex() + 1
	total := len(playlist)
	position := fmt.Sprintf("[%d/%d]", curIndex, total)

	// 音质：实际优先，配置兜底
	qualityName := player.CurQuality()
	if qualityName == "" {
		qualityName = qualityDisplayName(configs.AppConfig.Player.SongLevel)
	}

	text := fmt.Sprintf(" · %s · %s", position, qualityName)
	if detail := formatBitrateAndSize(player.CurFileSize(), song.Duration); detail != "" {
		text += " · " + detail
	}

	statusTextStyle := style.CurrentStyleSet().StatusBarText
	return statusTextStyle.Foreground(util.GetPrimaryColor()).Render(musicfoxStatusBarLabel) +
		statusTextStyle.Render(text)
}

// formatBitrateAndSize 返回「码率 · 文件大小」片段，无法计算时返回空串。
//
// 码率按 size*8/duration 估算，取整到 kbps；文件大小按 1024 进制
// 显示为 KB/MB，保留一位小数。
func formatBitrateAndSize(size int64, duration time.Duration) string {
	if size <= 0 || duration <= 0 {
		return ""
	}

	seconds := duration.Seconds()
	if seconds <= 0 {
		return ""
	}
	kbps := int(float64(size) * 8 / seconds / 1000)
	if kbps <= 0 {
		return ""
	}

	return fmt.Sprintf("%d kbps · %s", kbps, formatFileSize(size))
}

// formatFileSize 把字节数格式化为便于阅读的大小。
func formatFileSize(size int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
	)
	switch {
	case size >= mb:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(mb))
	case size >= kb:
		return fmt.Sprintf("%.0f KB", float64(size)/float64(kb))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

type queueQualityStatusBarComponent struct {
	player  *Player
	openURL func(string) error
}

func (c *queueQualityStatusBarComponent) View(*model.App, *model.Main) string {
	return formatQueueAndQuality(c.player)
}

func (c *queueQualityStatusBarComponent) HandleMouse(mouse tea.Mouse, x, _ int) (bool, tea.Cmd) {
	if mouse.Button != tea.MouseLeft || !c.IsMouseOver(x, 0) {
		return false, nil
	}
	return true, func() tea.Msg {
		if err := c.openURL(types.AppGithubUrl); err != nil {
			slog.Error("Failed to open musicfox GitHub repository", "url", types.AppGithubUrl, "err", err)
		}
		return nil
	}
}

func (c *queueQualityStatusBarComponent) IsMouseOver(x, _ int) bool {
	return x >= 0 && x < len(musicfoxStatusBarLabel)
}

// NewQueueQualityStatusBar 创建带队列位置与音质中间文本的 DefaultStatusBar。
func NewQueueQualityStatusBar(player *Player) *model.DefaultStatusBar {
	return newQueueQualityStatusBar(player, open.Start)
}

func newQueueQualityStatusBar(player *Player, openURL func(string) error) *model.DefaultStatusBar {
	return &model.DefaultStatusBar{
		Components: []model.StatusBarComponent{
			&queueQualityStatusBarComponent{player: player, openURL: openURL},
		},
	}
}
