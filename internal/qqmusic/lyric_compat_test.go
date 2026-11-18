package qqmusic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-musicfox/go-musicfox/internal/lyric"
)

// TestQRCFeedsParseYRC 是渠道兼容性的关键断言：
// QRC 解密并剥离 XML 后，必须能被 internal/lyric.ParseYRC 解析出逐字行。
//
// 这验证了「QRC 词元 (time,duration) 与 YRC 解析器的 (time,duration,0) 兼容」这一结论。
func TestQRCFeedsParseYRC(t *testing.T) {
	cipher := loadQRCFixture(t)
	decrypted, err := QrcDecrypt(cipher)
	if err != nil {
		t.Fatalf("QrcDecrypt 失败: %v", err)
	}

	yrc := qrcToYRC(decrypted)
	if yrc == "" {
		t.Fatal("qrcToYRC 返回空串")
	}

	lines, err := lyric.ParseYRC(yrc)
	if err != nil {
		t.Fatalf("ParseYRC 失败: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("ParseYRC 未解析出任何逐字行")
	}

	// 至少有一行带词级时间戳。
	var withWords int
	for _, line := range lines {
		if len(line.Words) > 0 {
			withWords++
		}
	}
	if withWords == 0 {
		t.Fatal("所有行都没有词级时间戳，逐字歌词未生效")
	}

	// 逐字时间必须单调递增（QRC 的 (time,duration) 是绝对毫秒）。
	first := lines[0]
	if len(first.Words) < 2 {
		t.Fatalf("首行词数过少: %d", len(first.Words))
	}
	for i := 1; i < len(first.Words); i++ {
		if first.Words[i].StartTime < first.Words[i-1].StartTime {
			t.Errorf("首行词时间非单调: word[%d].Start=%d < word[%d].Start=%d",
				i, first.Words[i].StartTime, i-1, first.Words[i-1].StartTime)
		}
	}

	// 词的 EndTime 必须大于 StartTime（时长被正确解析）。
	for i, w := range first.Words {
		if w.EndTime <= w.StartTime {
			t.Errorf("word[%d] %q EndTime=%d <= StartTime=%d", i, w.Word, w.EndTime, w.StartTime)
		}
	}

	t.Logf("解析出 %d 行，其中 %d 行带逐字时间；首行 %d 个词，起始 %dms",
		len(lines), withWords, len(first.Words), first.StartTime)
}

// TestQRCFixtureExists 保证夹具存在，避免测试静默跳过。
func TestQRCFixtureExists(t *testing.T) {
	path := filepath.Join("testdata", "qrc_sample.txt")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("夹具缺失: %v", err)
	}
	if info.Size() < 1000 {
		t.Errorf("夹具过小（%d 字节），可能未正确抓取", info.Size())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取夹具失败: %v", err)
	}
	cipher := strings.TrimSpace(string(data))
	if len(cipher)%8 != 0 {
		t.Errorf("夹具密文长度 %d 不是 8 的倍数", len(cipher))
	}
}
