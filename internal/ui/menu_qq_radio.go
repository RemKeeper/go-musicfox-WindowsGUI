package ui

import (
	"time"

	"github.com/anhoder/foxful-cli/model"

	"github.com/go-musicfox/go-musicfox/internal/structs"
	"github.com/go-musicfox/go-musicfox/utils/menux"
)

// qqGuessRadioMenu 是 QQ 音乐的「猜你喜欢」。
//
// 对应网易云的私人 FM：进入时自动拉取一批推荐并播放，
// 滚动到底部时自动追加下一批，形成无限随机推送。
//
// 底层用 QQ 的「猜你喜欢」电台（id=99）。实测该接口每次调用都返回
// 不同的 5 首歌，服务端按账号听歌画像实时挑选，无需客户端传种子。
// 单次上限 5 首，因此这里每次追加都发一次请求。
type qqGuessRadioMenu struct {
	baseMenu
	menus []model.MenuItem
	songs []structs.Song
	// loading 防止快速滚动时并发请求导致重复追加。
	loading bool
}

// NewQQGuessRadioMenu 创建「猜你喜欢」菜单。
func NewQQGuessRadioMenu(base baseMenu) *qqGuessRadioMenu {
	return &qqGuessRadioMenu{baseMenu: base}
}

func (m *qqGuessRadioMenu) IsSearchable() bool { return true }

func (m *qqGuessRadioMenu) IsPlayable() bool { return true }

func (m *qqGuessRadioMenu) GetMenuKey() string { return "qq_guess_radio" }

func (m *qqGuessRadioMenu) MenuViews() []model.MenuItem { return m.menus }

func (m *qqGuessRadioMenu) Songs() []structs.Song { return m.songs }

func (m *qqGuessRadioMenu) BeforeEnterMenuHook() model.Hook {
	return func(main *model.Main) (bool, model.Page) {
		if m.netease.qqClient == nil {
			m.netease.NotifyError("QQ 音乐渠道未初始化", "请检查配置")
			return false, nil
		}
		// 已有数据则直接进入，避免每次返回都重新拉取。
		if len(m.menus) > 0 && len(m.songs) > 0 {
			return true, nil
		}
		if err := m.appendBatch(); err != nil {
			m.netease.NotifyError("加载猜你喜欢失败", err.Error())
			return false, nil
		}
		if len(m.menus) == 0 {
			m.netease.NotifyInfo("猜你喜欢", "暂无推荐，请先多听几首歌")
			return false, nil
		}
		return true, nil
	}
}

// BottomOutHook 在滚动到底部时追加下一批推荐。
func (m *qqGuessRadioMenu) BottomOutHook() model.Hook {
	return func(main *model.Main) (bool, model.Page) {
		if m.loading {
			return false, nil
		}
		if err := m.appendBatch(); err != nil {
			// 追加失败不打断播放，仅提示。
			m.netease.NotifyError("追加推荐失败", err.Error())
			return false, nil
		}
		return true, nil
	}
}

// appendBatch 拉取一批推荐并追加到队列。
func (m *qqGuessRadioMenu) appendBatch() error {
	m.loading = true
	defer func() { m.loading = false }()

	qqSongs, err := m.netease.qqClient.GetGuessRecommend(5)
	if err != nil {
		return err
	}

	songs := toStructSongs(qqSongs)
	if len(songs) == 0 {
		return nil
	}

	m.songs = append(m.songs, songs...)
	m.menus = append(m.menus, menux.GetViewFromSongs(songs)...)

	// 同步到播放列表，使新增歌曲可被连续播放。
	if m.netease.player != nil && m.netease.player.playlistManager != nil {
		_ = m.netease.player.playlistManager.Initialize(
			m.netease.player.CurSongIndex(), m.songs)
		m.netease.player.playlistUpdateAt = time.Now()
	}
	return nil
}

// qqRadarMenu 是 QQ 音乐的「雷达推荐」。
//
// 与「猜你喜欢」互补：雷达推荐按页稳定返回（同一页内容固定），
// 适合「查看更多」式的浏览，而不是无限随机推送。
type qqRadarMenu struct {
	baseMenu
	menus   []model.MenuItem
	songs   []structs.Song
	page    int
	hasMore bool
	loading bool
}

// NewQQRadarMenu 创建「雷达推荐」菜单。
func NewQQRadarMenu(base baseMenu) *qqRadarMenu {
	return &qqRadarMenu{baseMenu: base, page: 1, hasMore: true}
}

func (m *qqRadarMenu) IsSearchable() bool { return true }

func (m *qqRadarMenu) IsPlayable() bool { return true }

func (m *qqRadarMenu) GetMenuKey() string { return "qq_radar" }

func (m *qqRadarMenu) MenuViews() []model.MenuItem { return m.menus }

func (m *qqRadarMenu) Songs() []structs.Song { return m.songs }

func (m *qqRadarMenu) BeforeEnterMenuHook() model.Hook {
	return func(main *model.Main) (bool, model.Page) {
		if m.netease.qqClient == nil {
			m.netease.NotifyError("QQ 音乐渠道未初始化", "请检查配置")
			return false, nil
		}
		if len(m.menus) > 0 && len(m.songs) > 0 {
			return true, nil
		}
		m.page = 1
		if err := m.fetch(); err != nil {
			m.netease.NotifyError("加载雷达推荐失败", err.Error())
			return false, nil
		}
		if len(m.menus) == 0 {
			m.netease.NotifyInfo("雷达推荐", "暂无推荐，请先多听几首歌")
			return false, nil
		}
		return true, nil
	}
}

func (m *qqRadarMenu) BottomOutHook() model.Hook {
	if !m.hasMore {
		return nil
	}
	return func(main *model.Main) (bool, model.Page) {
		if m.loading {
			return false, nil
		}
		m.page++
		if err := m.fetch(); err != nil {
			m.page--
			return false, nil
		}
		return true, nil
	}
}

func (m *qqRadarMenu) fetch() error {
	m.loading = true
	defer func() { m.loading = false }()

	qqSongs, hasMore, err := m.netease.qqClient.GetRadarRecommend(m.page)
	if err != nil {
		return err
	}
	m.hasMore = hasMore

	songs := toStructSongs(qqSongs)
	if len(songs) == 0 {
		m.hasMore = false
		return nil
	}

	m.songs = append(m.songs, songs...)
	m.menus = append(m.menus, menux.GetViewFromSongs(songs)...)

	if m.netease.player != nil && m.netease.player.playlistManager != nil {
		_ = m.netease.player.playlistManager.Initialize(
			m.netease.player.CurSongIndex(), m.songs)
		m.netease.player.playlistUpdateAt = time.Now()
	}
	return nil
}
