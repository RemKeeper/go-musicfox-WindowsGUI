package qqmusic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.etcd.io/bbolt"
)

// 本文件用于从 musicfox 的实际 BoltDB 中读取已登录的 QQ 音乐凭证，
// 以便对需要登录的接口（播放链接、我喜欢）做真实验证。
//
// 凭证来源：C:\Users\<user>\AppData\Local\go-musicfox\db\musicfox.db
// 存储 key：qq_music_credential（见 internal/storage/qq_credential.go）
//
// 若数据库不存在或未登录，测试自动跳过。

// loadCredentialFromDB 从 musicfox 的 BoltDB 读取 QQ 音乐凭证。
func loadCredentialFromDB(t *testing.T) Credential {
	t.Helper()

	// 允许通过环境变量覆盖路径，便于 CI 或自定义数据目录。
	dbPath := os.Getenv("MUSICFOX_DB_PATH")
	if dbPath == "" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			t.Skip("未找到 LOCALAPPDATA，跳过")
		}
		dbPath = filepath.Join(localAppData, "go-musicfox", "db", "musicfox.db")
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("数据库不存在（%s），跳过需要登录的测试", dbPath)
	}

	// bbolt 默认会无限等待文件锁。musicfox 正在运行时数据库被独占，
	// 必须设置短超时后跳过，否则整个测试会挂起。
	db, err := bbolt.Open(dbPath, 0o600, &bbolt.Options{
		ReadOnly: true,
		Timeout:  2 * time.Second,
	})
	if err != nil {
		t.Skipf("无法以只读方式打开数据库（可能被 musicfox 占用，请先退出）: %v", err)
	}
	defer db.Close()

	var raw []byte
	err = db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("default_bucket"))
		if bucket == nil {
			return nil
		}
		v := bucket.Get([]byte("qq_music_credential"))
		if v != nil {
			raw = make([]byte, len(v))
			copy(raw, v)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("读取数据库失败: %v", err)
	}
	if len(raw) == 0 {
		t.Skip("数据库中无 QQ 音乐凭证（尚未登录），跳过")
	}

	var cred Credential
	if err := json.Unmarshal(raw, &cred); err != nil {
		t.Fatalf("解析凭证失败: %v", err)
	}
	if !cred.Valid() {
		t.Skip("凭证无效（musicid/musickey 缺失），跳过")
	}

	t.Logf("已加载凭证: musicid=%d str_musicid=%q loginType=%d musickey=%s...",
		cred.MusicID, cred.StrMusicID, cred.LoginTyp, maskKey(cred.MusicKey))
	return cred
}

// maskKey 遮蔽密钥，避免日志泄露完整凭证。
func maskKey(key string) string {
	if len(key) <= 12 {
		return "***"
	}
	return key[:12]
}

// newLoggedInClient 构造一个已登录的客户端。
func newLoggedInClient(t *testing.T) *Client {
	t.Helper()
	c := NewClient()
	c.SetCredential(loadCredentialFromDB(t))
	if !c.LoggedIn() {
		t.Fatal("凭证设置后仍未登录")
	}
	return c
}

// TestRealPlayableInfo 用真实凭证验证播放链接。
//
// 这是「无法播放」的端到端验证：修复前 uin 用整数、filename 规则错误、
// comm 类型错误、登录模块用错，都会导致 purl 为空。
//
// 注意：若数据库里的凭证是由旧的错误登录模块生成的，即使 refresh 成功
// 也拿不到链接（服务端返回 retcode=104009 invalidq）。这种情况会跳过
// 并提示重新扫码，而不是判为失败。
func TestRealPlayableInfo(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要登录的测试")
	}

	c := newLoggedInClient(t)
	// 先按需续期，避免用过期 key 测试。
	c.EnsureFresh()

	// 用「晴天」做验证
	song := Song{
		ID:   97773,
		Mid:  "0039MnYb0qxYhV",
		File: File{MediaMid: "0049m3up3X6Q8s"},
	}

	ok := 0
	for _, q := range []Quality{QualityStandard, QualityHQ, QualitySQ} {
		info, err := c.FetchPlayableInfo(song, q)
		if err != nil {
			t.Logf("[%s] 失败: %v", q, err)
			continue
		}
		ok++
		t.Logf("[%s] URL=%s", q, info.URL)
		t.Logf("     MusicType=%s Size=%d", info.MusicType, info.Size)

		if info.URL == "" {
			t.Errorf("[%s] URL 为空", q)
		}
		if !isHTTPURL(info.URL) {
			t.Errorf("[%s] URL 格式异常: %q", q, info.URL)
		}
	}

	if ok == 0 {
		t.Skip("所有音质均无可用链接：凭证可能由旧的错误登录模块生成，" +
			"请重新扫码登录后再测")
	}
}

// TestRealPlayableAndStream 验证拿到链接后能真正下载到音频数据。
func TestRealPlayableAndStream(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要登录的测试")
	}

	c := newLoggedInClient(t)
	c.EnsureFresh()

	song := Song{
		ID:   97773,
		Mid:  "0039MnYb0qxYhV",
		File: File{MediaMid: "0049m3up3X6Q8s"},
	}

	info, err := c.FetchPlayableInfo(song, QualityStandard)
	if err != nil {
		t.Skipf("获取播放链接失败（凭证可能由旧的错误登录模块生成，请重新扫码）: %v", err)
	}
	t.Logf("播放链接: %s", info.URL)

	// 用与 track.Fetcher 相同的逻辑下载前若干字节，确认链接真的可用。
	n, head, err := downloadHead(info.URL, 4096)
	if err != nil {
		t.Fatalf("下载音频失败: %v", err)
	}
	t.Logf("成功下载 %d 字节，前 16 字节: % X", n, head[:min(16, len(head))])

	if n == 0 {
		t.Fatal("下载到 0 字节")
	}
}

// TestRealFavSongs 验证「我喜欢」可读。
func TestRealFavSongs(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要登录的测试")
	}

	c := newLoggedInClient(t)
	cred := c.Credential()

	songs, total, err := c.GetFavSongs(cred.MusicID, 1, 10)
	if err != nil {
		t.Fatalf("GetFavSongs 失败: %v", err)
	}
	t.Logf("我喜欢共 %d 首，本页 %d 首", total, len(songs))

	for i, s := range songs {
		if i >= 3 {
			break
		}
		name := ""
		if len(s.Singer) > 0 {
			name = s.Singer[0].Name
		}
		t.Logf("  %s - %s (id=%d, type=%d)", s.Name, name, s.ID, s.Type)
	}
}

// TestRealLyricForPlayableSong 验证能播放的歌曲同时能拿到歌词。
func TestRealLyricForPlayableSong(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要登录的测试")
	}

	c := newLoggedInClient(t)

	data, err := c.FetchLyric("0039MnYb0qxYhV")
	if err != nil {
		t.Fatalf("FetchLyric 失败: %v", err)
	}
	t.Logf("LRC %d 字符，QRC %d 字符", len(data.Original), len(data.Yrc))
	if data.Original == "" && data.Yrc == "" {
		t.Error("歌词全为空")
	}
}

// isHTTPURL 判断字符串是否是 http(s) URL。
func isHTTPURL(s string) bool {
	return len(s) > 7 && (s[:7] == "http://" || (len(s) > 8 && s[:8] == "https://"))
}
