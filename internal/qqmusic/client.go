package qqmusic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	// musicuURL 是无需签名的 CGI 入口，只读接口走这里。
	musicuURL = "https://u.y.qq.com/cgi-bin/musicu.fcg"
	// musicsURL 是签名 CGI 入口，搜索等风控接口必须走这里。
	musicsURL = "https://u.y.qq.com/cgi-bin/musics.fcg"

	// 风控返回码：换指纹重试即可恢复。
	riskControlCode = 2001
	// 请求重试上限。
	maxRetries = 4
	// 单次请求超时。
	requestTimeout = 15 * time.Second
)

// Platform 是请求使用的平台指纹。
type Platform uint8

const (
	// PlatformAndroid 用于搜索等强风控接口，必须带完整设备指纹。
	PlatformAndroid Platform = iota
	// PlatformWeb 用于大部分只读接口，无需设备指纹。
	PlatformWeb
)

// Credential 是 QQ 音乐登录态。
//
// 字段对齐 QQMusicApi models/request.py:Credential，保留刷新所需的
// 元信息（keyExpiresIn / musickeyCreateTime），以便判断是否需要续期。
type Credential struct {
	MusicID int64 `json:"musicid"`
	// StrMusicID 是登录响应里的 str_musicid，vkey 接口的 uin 参数用它。
	// 为空时回退到 MusicID 的十进制形式。
	StrMusicID string `json:"str_musicid,omitempty"`
	MusicKey   string `json:"musickey"`
	OpenID     string `json:"openid,omitempty"`
	UnionID    string `json:"unionid,omitempty"`
	EncryptU   string `json:"encryptUin,omitempty"`
	LoginTyp   int    `json:"loginType,omitempty"`
	// KeyExpiresIn 是 musickey 的有效期（秒）。
	KeyExpiresIn int `json:"keyExpiresIn,omitempty"`
	// KeyCreateAt 是 musickey 的签发时间（Unix 秒）。
	KeyCreateAt int64 `json:"musickeyCreateTime,omitempty"`
}

// Valid 报告凭证是否可用于需要登录的接口。
func (c Credential) Valid() bool {
	return c.MusicID > 0 && c.MusicKey != ""
}

// Expired 报告 musickey 是否已过期。
//
// 缺少有效期信息时保守返回 false（交由服务端裁决），
// 避免把可用凭证误判为过期。
func (c Credential) Expired() bool {
	if c.KeyExpiresIn <= 0 || c.KeyCreateAt <= 0 {
		return false
	}
	return time.Now().Unix() >= c.KeyCreateAt+int64(c.KeyExpiresIn)
}

// Uin 返回接口所需的 uin 字符串。
//
// 优先使用 str_musicid：部分账号的 uin 带前导零或非纯数字，
// 用整数形式会导致 vkey 返回空链接。
func (c Credential) Uin() string {
	if c.StrMusicID != "" {
		return c.StrMusicID
	}
	if c.MusicID > 0 {
		return strconv.FormatInt(c.MusicID, 10)
	}
	return "0"
}

// Client 是 QQ 音乐 API 客户端。
type Client struct {
	http     *http.Client
	mu       sync.RWMutex
	cred     Credential
	platform Platform

	// 设备指纹，风控时轮换。
	guid     string
	qimei36  string
	androidS string

	// cdn 缓存音频 CDN 域名列表。
	cdn cdnCache
}

// NewClient 创建客户端。
func NewClient() *Client {
	c := &Client{
		http: &http.Client{
			Timeout: requestTimeout,
			Transport: &http.Transport{
				// 复用长连接，降低 musics.fcg 的 TLS 断连概率。
				MaxIdleConns:        16,
				MaxIdleConnsPerHost: 8,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		platform: PlatformWeb,
	}
	c.rotateFingerprint()
	return c
}

// SetCredential 设置登录凭证。
func (c *Client) SetCredential(cred Credential) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cred = cred
}

// Credential 返回当前凭证。
func (c *Client) Credential() Credential {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cred
}

// LoggedIn 报告是否已登录。
func (c *Client) LoggedIn() bool {
	return c.Credential().Valid()
}

// rotateFingerprint 重新生成一套设备指纹。
//
// 实测只需随机 hex 即可通过风控，无需实现完整的 QIMEI 上报链路。
func (c *Client) rotateFingerprint() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.guid = randHex(32)
	c.qimei36 = randHex(36)
	c.androidS = randHex(32)
}

// deviceGUID 返回当前设备标识。
//
// 必须与 comm 里的 OpenUDID / udid 使用同一个值：vkey 等接口的 guid
// 参数若与 comm 不一致，风控会判定设备不可信，返回 retcode=104009
// (invalidq)，表现为「无法播放」。
func (c *Client) deviceGUID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.guid
}

// randHex 生成 n 位随机小写十六进制字符串。
func randHex(n int) string {
	const digits = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = digits[rand.IntN(len(digits))]
	}
	return string(b)
}

// cgiRequest 是一次 CGI 调用的描述。
type cgiRequest struct {
	Module       string         `json:"module"`
	Method       string         `json:"method"`
	Param        map[string]any `json:"param"`
	Comm         map[string]any `json:"-"`
	Sign         bool           `json:"-"`
	Cred         *Credential    `json:"-"`
	PreserveBool bool           `json:"-"`
	// Android 强制使用 Android 平台的 comm。
	//
	// vkey 等接口在 Web 平台下即使带 uin/g_tk 也拿不到 purl，
	// 必须用 Android 的 qq/authst 鉴权。
	Android bool `json:"-"`
}

// cgiEnvelope 是 CGI 请求体。
type cgiEnvelope struct {
	Comm map[string]any `json:"comm"`
	Req0 cgiRequest     `json:"req_0"`
}

// cgiResponse 是 CGI 响应体。
type cgiResponse struct {
	Code int `json:"code"`
	Req0 struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	} `json:"req_0"`
}

// buildComm 构造 comm 参数。
//
// 所有值都以字符串形式发送：QQ 接口对类型敏感，uin / g_tk 传整数
// 会导致 vkey 返回空 purl（表现为「无法播放」）。
func (c *Client) buildComm(platform Platform, cred Credential) map[string]any {
	c.mu.RLock()
	guid, qimei36, aid := c.guid, c.qimei36, c.androidS
	c.mu.RUnlock()

	if platform == PlatformAndroid {
		comm := map[string]any{
			"ct":         "11",
			"cv":         "20090008",
			"v":          "20090008",
			"chid":       "10003505",
			"tmeAppID":   "qqmusic",
			"traceid":    fmt.Sprintf("10002_%s_%d", guid, time.Now().Unix()),
			"QIMEI36":    qimei36,
			"OpenUDID":   guid,
			"udid":       guid,
			"OpenUDID2":  guid,
			"aid":        aid,
			"os_ver":     "12",
			"phonetype":  "Pixel 6",
			"format":     "json",
			"inCharset":  "utf-8",
			"outCharset": "utf-8",
		}
		if cred.Valid() {
			comm["qq"] = cred.Uin()
			comm["authst"] = cred.MusicKey
			if cred.LoginTyp != 0 {
				comm["tmeLoginType"] = strconv.Itoa(cred.LoginTyp)
			}
		}
		return comm
	}

	uin := "0"
	if cred.MusicID > 0 {
		uin = cred.Uin()
	}
	gtk := strconv.Itoa(gTK(cred.MusicKey))
	return map[string]any{
		"ct":                "24",
		"cv":                "4747474",
		"format":            "json",
		"inCharset":         "utf-8",
		"outCharset":        "utf-8",
		"notice":            "0",
		"need_new_code":     "1",
		"uin":               uin,
		"g_tk":              gtk,
		"g_tk_new_20200303": gtk,
		"platform":          "yqq.json",
	}
}

// gTK 计算 g_tk（hash33，初值 5381）。
func gTK(key string) int {
	if key == "" {
		return 5381
	}
	h := uint32(5381)
	for _, ch := range key {
		h = (h << 5) + h + uint32(ch)
	}
	return int(h & 0x7FFFFFFF)
}

// Do 执行一次 CGI 调用，返回 req_0.data 的原始 JSON。
//
// 内置重试：TLS 断连 / 超时 / 风控 code=2001 时换指纹重试。
func (c *Client) Do(req cgiRequest) (json.RawMessage, error) {
	cred := c.Credential()
	if req.Cred != nil {
		cred = *req.Cred
	}

	platform := c.platform
	if req.Sign || req.Android {
		platform = PlatformAndroid
	}

	lastErr := error(nil)
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 600 * time.Millisecond)
			c.rotateFingerprint()
		}

		data, err := c.doOnce(req, platform, cred)
		if err != nil {
			lastErr = err
			continue
		}
		return data, nil
	}
	return nil, fmt.Errorf("qqmusic: %s.%s 重试 %d 次仍失败: %w", req.Module, req.Method, maxRetries, lastErr)
}

// doOnce 执行单次请求。
func (c *Client) doOnce(req cgiRequest, platform Platform, cred Credential) (json.RawMessage, error) {
	comm := c.buildComm(platform, cred)
	if req.Comm != nil {
		for k, v := range req.Comm {
			comm[k] = v
		}
	}

	// guid 必须在每次请求时从当前设备指纹取，不能由调用方预先固定：
	// Do 的重试会轮换指纹，若 param 里的 guid 与 comm 的 OpenUDID
	// 不一致，风控会判定设备不可信（retcode=104009 / result=104003）。
	param := req.Param
	if param != nil {
		if _, ok := param["guid"]; ok {
			clone := make(map[string]any, len(param))
			for k, v := range param {
				clone[k] = v
			}
			clone["guid"] = c.deviceGUID()
			param = clone
		}
	}

	body := cgiEnvelope{
		Comm: comm,
		Req0: cgiRequest{
			Module: req.Module,
			Method: req.Method,
			Param:  param,
		},
	}

	// 必须与签名输入逐字节一致：不转义非 ASCII、无多余空格。
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("qqmusic: 编码请求体失败: %w", err)
	}

	url := musicuURL
	if req.Sign {
		url = fmt.Sprintf("%s?_=%d&sign=%s", musicsURL, time.Now().UnixMilli(), Sign(raw))
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("qqmusic: 构造请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if req.Sign {
		httpReq.Header.Set("User-Agent", "QQMusic 20090008(android 12)")
	} else {
		httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		httpReq.Header.Set("Referer", "https://y.qq.com/")
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qqmusic: HTTP %d", resp.StatusCode)
	}

	var envelope cgiResponse
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("qqmusic: 解析响应失败: %w", err)
	}
	if envelope.Code != 0 {
		return nil, fmt.Errorf("qqmusic: 外层 code=%d", envelope.Code)
	}
	if envelope.Req0.Code == riskControlCode {
		return nil, errRiskControl
	}
	if envelope.Req0.Code != 0 {
		return nil, &APIError{Code: envelope.Req0.Code}
	}
	// 部分接口（如 music.musicasset.PlaylistDetailWrite 的写操作）
	// 返回扁平结构，没有 req_0 外壳，业务字段直接在顶层。
	// 此时 Req0.Data 为空，应把整个响应体交给调用方解析。
	if len(envelope.Req0.Data) == 0 {
		return data, nil
	}
	return envelope.Req0.Data, nil
}
