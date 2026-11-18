package ui

import (
	"testing"

	"github.com/go-musicfox/go-musicfox/internal/structs"
)

// TestUserPlaylistSubMenuNoPanic 是「网易云歌单进不去」的回归测试。
//
// 历史 bug：边界检查写成 `len(m.playlists) < index`，
// 当 playlists 为空（未登录或尚未加载）且 index=0 时，
// `0 < 0` 为 false，继续执行 m.playlists[0] 触发
// `index out of range [0] with length 0` panic，
// 表现为「点歌单没反应 / 程序崩溃」。
func TestUserPlaylistSubMenuNoPanic(t *testing.T) {
	app, n, _ := setupSwitchSourceTestApp(t)
	n.SetSource(SourceNetease)

	main := app.MustMain()
	mm, ok := main.CurMenu().(*MainMenu)
	if !ok {
		t.Fatalf("当前菜单不是 MainMenu: %T", main.CurMenu())
	}

	// 「我的歌单」是第 3 项
	up, ok := mm.menuList[2].(*UserPlaylistMenu)
	if !ok {
		t.Fatalf("第 3 项不是 UserPlaylistMenu: %T", mm.menuList[2])
	}

	// 空列表时点击任意下标都不应 panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("SubMenu 发生 panic: %v", r)
		}
	}()

	for _, idx := range []int{-1, 0, 1, 100} {
		if sub := up.SubMenu(app, idx); sub != nil {
			t.Errorf("空列表时 SubMenu(%d) = %v, want nil", idx, sub)
		}
	}
}

// TestUserPlaylistSubMenuWithData 校验有数据时能正确返回详情菜单。
func TestUserPlaylistSubMenuWithData(t *testing.T) {
	app, n, _ := setupSwitchSourceTestApp(t)
	n.SetSource(SourceNetease)

	main := app.MustMain()
	mm, _ := main.CurMenu().(*MainMenu)
	up, ok := mm.menuList[2].(*UserPlaylistMenu)
	if !ok {
		t.Skip("第 3 项不是 UserPlaylistMenu")
	}

	// 注入测试数据
	up.playlists = []structs.Playlist{
		{Id: 111, Name: "歌单A"},
		{Id: 222, Name: "歌单B"},
	}

	if sub := up.SubMenu(app, 0); sub == nil {
		t.Error("SubMenu(0) 应返回详情菜单")
	}
	if sub := up.SubMenu(app, 1); sub == nil {
		t.Error("SubMenu(1) 应返回详情菜单")
	}
	// 越界仍应返回 nil
	if sub := up.SubMenu(app, 2); sub != nil {
		t.Errorf("SubMenu(2) 越界应返回 nil，实际 %v", sub)
	}
	if sub := up.SubMenu(app, -1); sub != nil {
		t.Errorf("SubMenu(-1) 越界应返回 nil，实际 %v", sub)
	}
}

// TestAlbumMenusSubMenuNoPanic 校验专辑类菜单的边界检查。
//
// 这些菜单原先有同样的 `len(m.albums) < index` 缺陷，一并修复。
func TestAlbumMenusSubMenuNoPanic(t *testing.T) {
	app, n, _ := setupSwitchSourceTestApp(t)
	n.SetSource(SourceNetease)

	main := app.MustMain()
	mm, _ := main.CurMenu().(*MainMenu)

	// 遍历所有菜单，对空列表的菜单调用 SubMenu 不应 panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("遍历菜单时发生 panic: %v", r)
		}
	}()

	for i := range mm.menuList {
		menu := mm.menuList[i]
		if menu == nil {
			continue
		}
		for _, idx := range []int{-1, 0, 1, 999} {
			_ = menu.SubMenu(app, idx)
		}
	}
}
