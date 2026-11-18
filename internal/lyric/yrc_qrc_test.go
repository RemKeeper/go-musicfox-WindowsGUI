package lyric

import (
	"testing"
)

// TestParseQRCFirstWordNotDropped 是「歌词缺第一个字」的回归测试。
//
// 历史 bug：QRC（QQ 音乐）的词元布局是 `词(起始,时长)`，词在括号**之前**；
// 而解析器只按 LRC 风格的 `(起始,时长,0)词`（词在括号之后）取词，
// 导致每行第一个词被丢弃，表现为「歌词缺第一个字」。
func TestParseQRCFirstWordNotDropped(t *testing.T) {
	// 真实 QRC 片段（《晴天》首行）
	line := "[0,2250]晴(0,160)天(160,160) (320,160)周(800,160)杰(960,160)伦(1120,160)"

	yrc := parseLRCStyleYRCLine(line)
	if yrc == nil {
		t.Fatal("解析失败")
	}

	var text string
	for _, w := range yrc.Words {
		text += w.Word
	}

	if text == "" {
		t.Fatal("未解析出任何词")
	}
	// 首字必须是「晴」
	if yrc.Words[0].Word != "晴" {
		t.Errorf("首个词 = %q, want \"晴\"（首字不应丢失）", yrc.Words[0].Word)
	}
	if text != "晴天 周杰伦" {
		t.Errorf("合并文本 = %q, want \"晴天 周杰伦\"", text)
	}

	// 时间必须递增且正确
	wantTimes := []int64{0, 160, 320, 800, 960, 1120}
	if len(yrc.Words) != len(wantTimes) {
		t.Fatalf("词数 = %d, want %d", len(yrc.Words), len(wantTimes))
	}
	for i, want := range wantTimes {
		if yrc.Words[i].StartTime != want {
			t.Errorf("词[%d] 起始 = %d, want %d", i, yrc.Words[i].StartTime, want)
		}
	}
}

// TestParseQRCLiteralParenthesis 校验歌词里的 `(` 不被误判为时间标签。
//
// QRC 中 `(` 可能属于歌词内容，并与后面的时间标签粘连成
// `((1440,160)`，此时第一个 `(` 必须作为文本保留。
func TestParseQRCLiteralParenthesis(t *testing.T) {
	line := "[0,2250]晴(0,160)天(160,160) (320,160)((1440,160)Jay(1600,160)"

	yrc := parseLRCStyleYRCLine(line)
	if yrc == nil {
		t.Fatal("解析失败")
	}

	var text string
	for _, w := range yrc.Words {
		text += w.Word
	}

	if text != "晴天 (Jay" {
		t.Errorf("合并文本 = %q, want \"晴天 (Jay\"", text)
	}

	// `(` 的时间应为 1440（来自其后的标签）
	var parenWord *YRCWord
	for i := range yrc.Words {
		if yrc.Words[i].Word == "(" {
			parenWord = &yrc.Words[i]
			break
		}
	}
	if parenWord == nil {
		t.Fatal("未找到 `(` 词元")
	}
	if parenWord.StartTime != 1440 {
		t.Errorf("`(` 起始 = %d, want 1440", parenWord.StartTime)
	}
}

// TestParseLRCStyleYRCStillWorks 校验网易云的 LRC 风格 YRC 未被破坏。
//
// 网易云布局：`(起始,时长,0)词`，词在括号之后。
func TestParseLRCStyleYRCStillWorks(t *testing.T) {
	line := "[1000,2000](1000,500,0)你(1500,500,0)好(2000,500,0)啊"

	yrc := parseLRCStyleYRCLine(line)
	if yrc == nil {
		t.Fatal("解析失败")
	}

	var text string
	for _, w := range yrc.Words {
		text += w.Word
	}

	if text != "你好啊" {
		t.Errorf("合并文本 = %q, want \"你好啊\"", text)
	}
	if len(yrc.Words) != 3 {
		t.Fatalf("词数 = %d, want 3", len(yrc.Words))
	}
	if yrc.Words[0].StartTime != 1000 {
		t.Errorf("首词起始 = %d, want 1000", yrc.Words[0].StartTime)
	}
}

// TestParseQRCAllLinesHaveFirstChar 校验多行 QRC 每行首字都不丢。
func TestParseQRCAllLinesHaveFirstChar(t *testing.T) {
	data := "[0,1000]第(0,200)一(200,200)行(400,200)\n" +
		"[1000,1000]第(1000,200)二(1200,200)行(1400,200)\n" +
		"[2000,1000]第(2000,200)三(2200,200)行(2400,200)"

	lines, err := ParseYRC(data)
	if err != nil {
		t.Fatalf("ParseYRC 失败: %v", err)
	}
	if len(lines) != 3 {
		t.Fatalf("行数 = %d, want 3", len(lines))
	}

	want := []string{"第一行", "第二行", "第三行"}
	for i, line := range lines {
		var text string
		for _, w := range line.Words {
			text += w.Word
		}
		if text != want[i] {
			t.Errorf("第 %d 行文本 = %q, want %q", i, text, want[i])
		}
	}
}

// TestScanTimingTag 校验时间标签的严格匹配。
func TestScanTimingTag(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		start    int
		wantOK   bool
		wantTime int64
		wantDur  int64
	}{
		{"标准两段", "(0,160)", 0, true, 0, 160},
		{"三段", "(0,160,0)", 0, true, 0, 160},
		{"非数字", "(abc,160)", 0, false, 0, 0},
		{"缺少时长", "(0)", 0, false, 0, 0},
		{"嵌套括号", "((1440,160)", 0, false, 0, 0},
		{"空括号", "()", 0, false, 0, 0},
		{"四段", "(0,160,0,1)", 0, false, 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, dur, _, ok := scanTimingTag(tc.content, tc.start)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok {
				if start != tc.wantTime {
					t.Errorf("start = %d, want %d", start, tc.wantTime)
				}
				if dur != tc.wantDur {
					t.Errorf("dur = %d, want %d", dur, tc.wantDur)
				}
			}
		})
	}
}
