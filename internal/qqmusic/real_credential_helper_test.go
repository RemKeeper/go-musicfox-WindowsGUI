package qqmusic

import (
	"io"
	"net/http"
	"time"
)

// downloadHead 下载 URL 的前 limit 字节，用于验证播放链接真的可用。
//
// 与 track.Fetcher.FetchStream 使用同样的方式发起请求，
// 因此能反映实际播放时的行为。
func downloadHead(url string, limit int64) (int, []byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, nil, &httpStatusError{Status: resp.Status}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return 0, nil, err
	}
	return len(body), body, nil
}

// httpStatusError 表示非 200 的 HTTP 响应。
type httpStatusError struct{ Status string }

func (e *httpStatusError) Error() string { return "HTTP " + e.Status }
