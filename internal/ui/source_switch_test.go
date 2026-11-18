package ui

import (
	"context"
	"io"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"

	"github.com/go-musicfox/go-musicfox/internal/configs"
	"github.com/go-musicfox/go-musicfox/internal/lastfm"
	"github.com/go-musicfox/go-musicfox/internal/storage"
	"github.com/go-musicfox/go-musicfox/utils/app"
)

// setupSwitchSourceTestApp 准备一个可渲染真实主菜单的测试环境。
//
// 主菜单的 Lastfm 项会调用 lastfm.Client.NeedAuth，而 lastfm.NewClient
// 依赖 storage.DBManager，因此需要把应用根目录指向临时目录并初始化 DB。
func setupSwitchSourceTestApp(t *testing.T) (*model.App, *Netease, *MainMenu) {
	t.Helper()

	previousConfig := configs.AppConfig
	configs.AppConfig = &configs.Config{}
	t.Cleanup(func() { configs.AppConfig = previousConfig })

	// 隔离数据目录，避免读写用户真实数据库。
	app.SetupPureRoot(t.TempDir())
	dbManager := new(storage.LocalDBManager)
	storage.DBManager = dbManager
	// 关闭数据库，否则 Windows 上 TempDir 清理会因文件占用失败。
	t.Cleanup(func() { _ = dbManager.Close() })

	opts := model.DefaultOptions()
	opts.EnableStartup = false
	opts.MainMenu = &switchSourceTestMenu{}
	opts.MainMenuTitle = &model.MenuItem{Title: "main"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opts.TeaOptions = []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
	}

	testApp := model.NewApp(opts)
	_ = testApp.Run()
	if testApp.Main() == nil {
		t.Fatal("main page was not initialized")
	}
	_, _ = testApp.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	n := &Netease{
		App:      testApp,
		source:   SourceNetease,
		qqClient: newQQClient(),
		lastfm:   lastfm.NewClient(),
	}
	menu := NewMainMenu(n)
	testApp.MustMain().EnterMenu(menu, &model.MenuItem{Title: "main"})

	return testApp, n, menu
}

// TestSwitchSourceDoesNotPanicOnSecondPage 是渠道切换 panic 的回归测试。
//
// 历史 bug：网易云菜单 17 项、QQ 音乐菜单 9 项，menuPageSize 为 10。
// 在网易云菜单翻到第 2 页（menuCurPage=2）后切换渠道，新菜单只有 9 项，
// View 计算 menuList[start:end] 得到 [10:9]，触发
// "slice bounds out of range [10:9]" 崩溃。
func TestSwitchSourceDoesNotPanicOnSecondPage(t *testing.T) {
	testApp, n, _ := setupSwitchSourceTestApp(t)

	// 把分页推到第 2 页（模拟用户在长菜单里翻页）。
	testApp.MustMain().MoveBottom()

	beforePage := testApp.MustMain().CurPage()
	if beforePage < 2 {
		t.Fatalf("前置条件不成立：期望停在第 2 页，实际第 %d 页（菜单项 %d 个）",
			beforePage, len(testApp.MustMain().MenuList()))
	}

	// 切到 QQ 音乐：菜单从 17 项缩到 9 项，旧分页会越界。
	n.SetSource(SourceQQ)

	if got := testApp.MustMain().CurPage(); got != 1 {
		t.Errorf("切换后应回到第 1 页，实际第 %d 页", got)
	}
	if got := testApp.MustMain().SelectedIndex(); got != 0 {
		t.Errorf("切换后选中项应归零，实际 %d", got)
	}

	// 关键断言：渲染不得 panic。
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("切换渠道后渲染 panic: %v", r)
		}
	}()
	_ = testApp.MustMain().View(testApp)
}

// TestSwitchSourceRoundTrip 校验来回切换都安全，且菜单项数量正确。
func TestSwitchSourceRoundTrip(t *testing.T) {
	testApp, n, menu := setupSwitchSourceTestApp(t)

	neteaseCount := len(menu.MenuViews())

	n.SetSource(SourceQQ)
	qqCount := len(menu.MenuViews())
	if qqCount >= neteaseCount {
		t.Errorf("QQ 菜单项应少于网易云：qq=%d netease=%d", qqCount, neteaseCount)
	}

	// 再切回去，应恢复原数量
	n.SetSource(SourceNetease)
	if got := len(menu.MenuViews()); got != neteaseCount {
		t.Errorf("切回网易云后菜单项 = %d, want %d", got, neteaseCount)
	}

	// 每次切换后渲染都不得 panic
	for _, src := range []Source{SourceQQ, SourceNetease, SourceQQ} {
		n.SetSource(src)
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("切换到 %s 后渲染 panic: %v", src, r)
				}
			}()
			_ = testApp.MustMain().View(testApp)
		}()
	}
}

// switchSourceTestMenu 是测试用的最小主菜单。
type switchSourceTestMenu struct {
	model.DefaultMenu
}

func (m *switchSourceTestMenu) GetMenuKey() string { return "switch_source_test" }

func (m *switchSourceTestMenu) MenuViews() []model.MenuItem {
	return []model.MenuItem{{Title: "main"}}
}

func (m *switchSourceTestMenu) SubMenu(_ *model.App, _ int) model.Menu { return nil }
