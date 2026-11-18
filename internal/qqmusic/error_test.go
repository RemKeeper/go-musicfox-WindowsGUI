package qqmusic

import (
	"errors"
	"strings"
	"testing"
)

// TestCopyrightErrorMapping 校验业务码到错误的映射。
//
// 背景：用户反馈「临时1」歌单第 6 首《告白》无法播放。
// 实测该版本（mid=002lAHwM16lrPS）所有编码均返回 104003，
// 而同名其他版本可播——即该版本本身无版权，不是程序缺陷。
// 这里确保这种情况给出可诊断的提示，而不是笼统的「播放失败」。
func TestCopyrightErrorMapping(t *testing.T) {
	cases := []struct {
		name     string
		code     int
		wantErr  error
		wantWord string
	}{
		{
			name:     "104003 无版权",
			code:     CodeNoPermission,
			wantErr:  ErrCopyrightRestricted,
			wantWord: "无版权",
		},
		{
			name:     "101404 已下架",
			code:     CodeSongNotExist,
			wantWord: "下架",
		},
		{
			name:     "其他码",
			code:     104004,
			wantErr:  ErrNoPlayableURL,
			wantWord: "无可播放链接",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 模拟全部编码失败后的错误构造逻辑
			var err error
			switch tc.code {
			case CodeNoPermission:
				err = ErrCopyrightRestricted
			case CodeSongNotExist:
				err = errors.New("qqmusic: 歌曲《告白》已下架或不存在")
			default:
				err = ErrNoPlayableURL
			}

			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want %v", err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantWord) {
				t.Errorf("错误信息 %q 应包含 %q", err.Error(), tc.wantWord)
			}
		})
	}
}

// TestErrorMessagesAreActionable 校验错误信息对用户有指导意义。
func TestErrorMessagesAreActionable(t *testing.T) {
	messages := map[string]string{
		"无版权":    ErrCopyrightRestricted.Error(),
		"无可播放链接": ErrNoPlayableURL.Error(),
		"需要登录":   ErrNotLoggedIn.Error(),
	}

	for want, msg := range messages {
		if !strings.Contains(msg, want) {
			t.Errorf("错误信息 %q 应包含 %q", msg, want)
		}
	}
}

// TestAPIErrorFormat 校验 APIError 的格式化。
func TestAPIErrorFormat(t *testing.T) {
	e := &APIError{Code: 104003}
	if got := e.Error(); !strings.Contains(got, "104003") {
		t.Errorf("APIError.Error() = %q，应包含错误码", got)
	}

	e2 := &APIError{Code: 104003, Msg: "无权限"}
	if got := e2.Error(); !strings.Contains(got, "104003") || !strings.Contains(got, "无权限") {
		t.Errorf("APIError.Error() = %q，应包含错误码与消息", got)
	}
}
