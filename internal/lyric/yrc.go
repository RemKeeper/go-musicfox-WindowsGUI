package lyric

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"unicode/utf8"
)

type YRCWord struct {
	Word      string `json:"word"`
	StartTime int64  `json:"startTime"` // 毫秒
	EndTime   int64  `json:"endTime"`   // 毫秒
}

// YRCLine 表示包含词语和可选翻译的完整 YRC 歌词行。
type YRCLine struct {
	StartTime       int64
	EndTime         int64
	Words           []YRCWord
	TranslatedLyric string // 整行的翻译文本
	RomanLyric      string // 整行的罗马音文本
	IsBG            bool   // 背景歌词标志
	IsDuet          bool   // 二重唱（与他人合唱）标志
}

// YRCLineProgress describes word-by-word playback at one instant.
type YRCLineProgress struct {
	CompletedWords  int
	CurrentWord     int
	CurrentProgress float64
}

// ProgressYRCLineAtTimeMs returns the completed word count and active word for timeMs.
// Gaps between words have no active word; words that have ended remain completed.
func ProgressYRCLineAtTimeMs(line YRCLine, timeMs int64) YRCLineProgress {
	progress := YRCLineProgress{CurrentWord: -1}
	for i, word := range line.Words {
		if timeMs < word.StartTime {
			break
		}
		if word.EndTime <= word.StartTime || timeMs >= word.EndTime {
			progress.CompletedWords = i + 1
			continue
		}

		progress.CurrentWord = i
		progress.CurrentProgress = float64(timeMs-word.StartTime) / float64(word.EndTime-word.StartTime)
		return progress
	}
	return progress
}

// ParseYRC解析API返回的YRC JSON字符串并返回YRCLine切片。
// YRC 格式有两种：
// 1. 纯 JSON：{"t":timestamp,"c":[{"tx":"word","tr":[offset,duration]}...]}
// 2. LRC 风格：[startTime,duration](wordTime,duration,0)word(wordTime,duration,0)word...
func ParseYRC(yrcData string) ([]YRCLine, error) {
	if yrcData == "" {
		return nil, nil
	}

	lines := strings.Split(yrcData, "\n")
	result := make([]YRCLine, 0, len(lines))

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 首先尝试解析为纯 JSON（用于作曲家、作词家等元数据行）
		if strings.HasPrefix(line, "{") {
			if yrcLine := parseJSONYRCLine(line); yrcLine != nil {
				result = append(result, *yrcLine)
				continue
			}
		}

		// 尝试解析为 LRC 风格格式 [time,duration](time,dur,0)word(time,dur,0)word...
		if strings.HasPrefix(line, "[") {
			if yrcLine := parseLRCStyleYRCLine(line); yrcLine != nil {
				result = append(result, *yrcLine)
				continue
			}
		}

		slog.Debug("failed to parse YRC line", "lineNum", lineNum, "preview", line[:min(80, len(line))])
	}

	// 按开始时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartTime < result[j].StartTime
	})

	return result, nil
}

// parseJSONYRCLine 解析纯 JSON 格式：{"t":time,"c":[{"tx":"word","tr":[offset,duration]}]}
func parseJSONYRCLine(line string) *YRCLine {
	var lineData struct {
		T int64 `json:"t"` // 开始时间（毫秒）
		C []struct {
			Tx string  `json:"tx"`           // 文本（词）
			Tr []int64 `json:"tr,omitempty"` // [起始偏移, 时长]
		} `json:"c"`
	}

	if err := json.Unmarshal([]byte(line), &lineData); err != nil {
		return nil
	}

	yrcLine := YRCLine{
		StartTime: lineData.T,
		Words:     make([]YRCWord, 0, len(lineData.C)),
	}

	var currentTime int64 = lineData.T
	for _, wordData := range lineData.C {
		word := YRCWord{
			Word:      wordData.Tx,
			StartTime: currentTime,
		}

		if len(wordData.Tr) >= 2 {
			word.StartTime = lineData.T + wordData.Tr[0]
			word.EndTime = word.StartTime + wordData.Tr[1]
			currentTime = word.EndTime
		} else {
			// 没有时间信息，估算
			word.EndTime = currentTime + 500 // 默认 500 毫秒
			currentTime = word.EndTime
		}

		yrcLine.Words = append(yrcLine.Words, word)
	}

	// 设置行的结束时间
	if len(yrcLine.Words) > 0 {
		yrcLine.EndTime = yrcLine.Words[len(yrcLine.Words)-1].EndTime
	} else {
		yrcLine.EndTime = yrcLine.StartTime
	}

	return &yrcLine
}

// parseLRCStyleYRCLine 解析 LRC 风格格式：[startTime,duration](wordTime,duration,0)word(wordTime,duration,0)word...
func parseLRCStyleYRCLine(line string) *YRCLine {
	// 提取开头的 [startTime,duration]
	closeBracket := strings.Index(line, "]")
	if closeBracket == -1 {
		return nil
	}

	// 解析行的时间 [startTime,duration]
	timingStr := line[1:closeBracket]
	timeParts := strings.Split(timingStr, ",")
	if len(timeParts) < 2 {
		return nil
	}

	var lineStartTime, lineDuration int64
	if _, err := fmt.Sscanf(timeParts[0], "%d", &lineStartTime); err != nil {
		return nil
	}
	if _, err := fmt.Sscanf(timeParts[1], "%d", &lineDuration); err != nil {
		return nil
	}

	yrcLine := YRCLine{
		StartTime: lineStartTime,
		EndTime:   lineStartTime + lineDuration,
		Words:     []YRCWord{},
	}

	// 解析词段。
	//
	// 支持两种词元布局：
	//   - LRC 风格 YRC：`(time,duration,0)word`（词在括号之后）
	//   - QRC（QQ 音乐）：`word(time,duration)`（词在括号之前）
	//
	// QRC 的词在括号前，若只按「括号后取词」解析，每行首个词会被丢弃，
	// 表现为「歌词缺第一个字」。
	content := line[closeBracket+1:]

	var pendingText string // 当前 `(` 之前累积的文本
	i := 0
	for i < len(content) {
		if content[i] != '(' {
			// 累积括号前的文本（QRC 布局的词）。
			// 必须按 rune 前进，否则多字节字符会被拆坏。
			_, size := utf8.DecodeRuneInString(content[i:])
			if size <= 0 {
				size = 1
			}
			pendingText += content[i : i+size]
			i += size
			continue
		}

		// 提取时间信息 (time,duration[,0])
		//
		// 必须只匹配「紧邻的合法时间标签」，不能用 strings.Index 找最远的 `)`：
		// QRC 里 `(` 本身可能是歌词内容，且会与后面的时间标签粘连成
		// `((1440,160)`，此时第一个 `(` 不是时间标签，应作为文本。
		wordTime, wordDuration, tagEnd, ok := scanTimingTag(content, i)
		if !ok {
			// 不是时间括号，把 `(` 当普通文本累积。
			pendingText += "("
			i++
			continue
		}
		closeParenIdx := tagEnd

		var wordText string
		if pendingText != "" {
			// QRC 布局：词在括号之前。
			wordText = pendingText
			pendingText = ""
			i = closeParenIdx + 1
		} else {
			// LRC 布局：词在括号之后，直到下一个 `(`。
			j := closeParenIdx + 1
			for j < len(content) && content[j] != '(' {
				j++
			}
			wordText = content[closeParenIdx+1 : j]
			i = j
		}

		if wordText == "" {
			continue
		}

		yrcLine.Words = append(yrcLine.Words, YRCWord{
			Word:      wordText,
			StartTime: wordTime,
			EndTime:   wordTime + wordDuration,
		})
	}

	// 行尾残留文本（没有时间标签）作为最后一词，时间沿用行结束时间。
	if pendingText != "" {
		yrcLine.Words = append(yrcLine.Words, YRCWord{
			Word:      pendingText,
			StartTime: yrcLine.EndTime,
			EndTime:   yrcLine.EndTime,
		})
	}

	if len(yrcLine.Words) == 0 {
		return nil
	}

	return &yrcLine
}

// scanTimingTag 从 content[start] 处的 `(` 开始，尝试解析一个时间标签。
//
// 合法格式：`(起始,时长)` 或 `(起始,时长,0)`，两个数字均为纯十进制。
// 成功返回时间、时长、`)` 的下标与 true；否则返回 false。
//
// 必须严格校验，避免把歌词里的 `(` 误当成时间标签
// （QRC 中常见 `((1440,160)` 这种粘连）。
func scanTimingTag(content string, start int) (startMs, durationMs int64, end int, ok bool) {
	if start >= len(content) || content[start] != '(' {
		return 0, 0, 0, false
	}

	// 找到最近的 `)`，且中间不能出现 `(`。
	i := start + 1
	for i < len(content) && content[i] != ')' {
		if content[i] == '(' {
			return 0, 0, 0, false
		}
		i++
	}
	if i >= len(content) {
		return 0, 0, 0, false
	}

	parts := strings.Split(content[start+1:i], ",")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, 0, 0, false
	}
	if !scanInt(parts[0], &startMs) || !scanInt(parts[1], &durationMs) {
		return 0, 0, 0, false
	}
	// 第三个字段（若存在）必须是数字，用于区分 `(0,160,0)` 与 `(0,160)x`。
	if len(parts) == 3 {
		var third int64
		if !scanInt(parts[2], &third) {
			return 0, 0, 0, false
		}
	}
	return startMs, durationMs, i, true
}

// scanInt 把十进制整数字符串解析到 out，成功返回 true。
//
// 比 fmt.Sscanf 更严格：拒绝空串、带符号或带后缀的输入，
// 用于判定括号内是否为合法的时间标签。
func scanInt(s string, out *int64) bool {
	if s == "" {
		return false
	}
	var v int64
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
		v = v*10 + int64(s[i]-'0')
	}
	*out = v
	return true
}

// parseTimeTag 将 LRC 时间戳如 "01:23.45" 解析为毫秒
func parseTimeTag(tag string) int64 {
	parts := strings.Split(tag, ":")
	if len(parts) != 2 {
		return -1
	}

	var min, sec int
	var ms int
	if _, err := fmt.Sscanf(parts[0], "%d", &min); err != nil {
		return -1
	}
	if strings.Contains(parts[1], ".") {
		if _, err := fmt.Sscanf(parts[1], "%d.%d", &sec, &ms); err != nil {
			return -1
		}
	} else {
		if _, err := fmt.Sscanf(parts[1], "%d", &sec); err != nil {
			return -1
		}
	}

	return int64(min)*60000 + int64(sec)*1000 + int64(ms)*10
}

// AlignTranslationToYRC 通过匹配时间将翻译的 LRC 行合并到 YRC 行中。
// 这将行级别的翻译文本（来自 ytlrc.lyric）与 YRC 词数据对齐。
func AlignTranslationToYRC(yrcLines []YRCLine, translatedLRC string) []YRCLine {
	if translatedLRC == "" || len(yrcLines) == 0 {
		return yrcLines
	}

	// 解析翻译的 LRC 以获取时间 -> 翻译文本的映射
	transFile, err := ReadTranslateLRC(strings.NewReader(translatedLRC))
	if err != nil || transFile == nil {
		slog.Debug("failed to parse translated LRC for YRC alignment", "error", err)
		return yrcLines
	}

	// 按开始时间将 YRC 行与翻译片段匹配
	for i := range yrcLines {
		if trans, ok := transFile.fragments[yrcLines[i].StartTime]; ok && trans != "" {
			yrcLines[i].TranslatedLyric = trans
		}
	}

	return yrcLines
}

// AlignRomanToYRC 通过匹配时间将罗马音 LRC 行合并到 YRC 行中。
func AlignRomanToYRC(yrcLines []YRCLine, romanLRC string) []YRCLine {
	if romanLRC == "" || len(yrcLines) == 0 {
		return yrcLines
	}

	// 解析罗马音 LRC 以获取时间 -> 罗马音文本的映射
	romanFile, err := ReadTranslateLRC(strings.NewReader(romanLRC))
	if err != nil || romanFile == nil {
		slog.Debug("failed to parse roman LRC for YRC alignment", "error", err)
		return yrcLines
	}

	// 按开始时间将 YRC 行与罗马音片段匹配
	for i := range yrcLines {
		if roman, ok := romanFile.fragments[yrcLines[i].StartTime]; ok && roman != "" {
			yrcLines[i].RomanLyric = roman
		}
	}

	return yrcLines
}

// AlignTranslationFragmentsToYRC 将已解析的 LRC 翻译片段（timeMs->text）合并到 YRC 行中。
// 用于没有 ytlrc，仅有传统 tlyric 的回退场景。
func AlignTranslationFragmentsToYRC(fragments []LRCFragment, yrcLines []YRCLine, transFragments map[int64]string) []YRCLine {
	if len(transFragments) == 0 || len(yrcLines) == 0 {
		return yrcLines
	}
	type processedFrag struct {
		content string
		timeMs  int64
	}
	procFragments := make([]processedFrag, len(fragments))
	for i, frag := range fragments {
		content := strings.TrimSpace(frag.Content)
		content = strings.TrimPrefix(content, "\ufeff")
		procFragments[i] = processedFrag{content: content, timeMs: frag.StartTimeMs}
	}

	for i := range yrcLines {
		var sb strings.Builder
		for _, w := range yrcLines[i].Words {
			sb.WriteString(w.Word)
		}
		lineText := strings.TrimSpace(sb.String())
		lineText = strings.TrimPrefix(lineText, "\ufeff")

		if lineText == "" {
			continue
		}

		var matchedTime int64 = -1
		for _, frag := range procFragments {
			if frag.content != "" && (lineText == frag.content || strings.Contains(frag.content, lineText) || strings.Contains(lineText, frag.content)) {
				matchedTime = frag.timeMs
				break
			}
		}

		if matchedTime != -1 {
			if trans, ok := transFragments[matchedTime]; ok && trans != "" {
				yrcLines[i].TranslatedLyric = trans
			}
		}
	}

	return yrcLines
}

// FindYRCLineAtTimeMs 查找给定时间（毫秒）处活动的 YRC 行索引。
// 如果没有活动行则返回 -1。
func FindYRCLineAtTimeMs(yrcLines []YRCLine, timeMs int64) int {
	for i := len(yrcLines) - 1; i >= 0; i-- {
		if timeMs >= yrcLines[i].StartTime {
			return i
		}
	}
	return -1
}

// FindYRCWordAtTimeMs 查找 YRC 行中给定时间（毫秒）处活动的词索引。
// 如果没有活动词则返回 -1。
func FindYRCWordAtTimeMs(words []YRCWord, timeMs int64) int {
	for i := len(words) - 1; i >= 0; i-- {
		if timeMs >= words[i].StartTime {
			return i
		}
	}
	return -1
}
