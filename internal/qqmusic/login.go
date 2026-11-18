package qqmusic

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// QRLoginType 是扫码登录方式。
type QRLoginType uint8

const (
	// QRLoginQQ 是手机 QQ 扫码授权。
	QRLoginQQ QRLoginType = iota
	// QRLoginWX 是微信扫码授权。
	QRLoginWX
)

// QRCode 是待扫描的二维码。
type QRCode struct {
	// Image 是二维码图片的二进制内容（PNG / JPEG）。
	Image []byte
	// MimeType 是图片类型。
	MimeType string
	// Identifier 是轮询所需的标识（qrsig / uuid）。
	Identifier string
	// Type 是登录方式。
	Type QRLoginType
}

// QRLoginEvent 是扫码状态事件。
type QRLoginEvent uint8

const (
	// QREventWaiting 等待扫码。
	QREventWaiting QRLoginEvent = iota
	// QREventScanned 已扫码，等待确认。
	QREventScanned
	// QREventConfirmed 已确认，正在登录。
	QREventConfirmed
	// QREventDone 登录成功。
	QREventDone
	// QREventRefused 用户拒绝。
	QREventRefused
	// QREventExpired 二维码过期。
	QREventExpired
)

// QRLoginResult 是一次轮询的结果。
type QRLoginResult struct {
	Event      QRLoginEvent
	Credential *Credential
	Err        error
}

// qqStatusRe 解析 QQ 的 ptuiCB(...) 响应。
var qqStatusRe = regexp.MustCompile(`ptuiCB\((.*?)\)`)

// qqArgsRe 提取 ptuiCB 参数中的单引号字符串。
var qqArgsRe = regexp.MustCompile(`'((?:\\.|[^'])*)'`)

var (
	qqSigxRe = regexp.MustCompile(`(?:\?|&)ptsigx=(.+?)&s_url`)
	qqUinRe  = regexp.MustCompile(`(?:\?|&)uin=(.+?)&service`)
	wxUUIDRe = regexp.MustCompile(`uuid=(.+?)"`)
	wxStatRe = regexp.MustCompile(`window\.wx_errcode=(\d+);window\.wx_code='([^']*)'`)
)

// loginHTTP 是登录专用的 HTTP 客户端，需要独立 CookieJar 保存登录中间态。
type loginHTTP struct {
	client *http.Client
}

// newLoginHTTP 创建带独立 CookieJar 的客户端。
func newLoginHTTP() *loginHTTP {
	jar, _ := cookiejar.New(nil)
	return &loginHTTP{
		client: &http.Client{
			Timeout: 35 * time.Second,
			Jar:     jar,
			// 登录流程需要读取中间跳转的 Cookie，禁止自动跟随重定向。
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// get 执行 GET 请求并返回响应。
func (l *loginHTTP) get(url string, headers map[string]string) (*http.Response, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, err
	}
	return resp, body, nil
}

// postForm 以 application/x-www-form-urlencoded 提交表单。
//
// 与 get 共用同一个 CookieJar，因此前一步种下的 p_skey 会自动带上。
func (l *loginHTTP) postForm(url string, form url.Values, headers map[string]string) (*http.Response, []byte, error) {
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, err
	}
	return resp, body, nil
}

// GetQRCode 获取指定方式的登录二维码。
func (c *Client) GetQRCode(loginType QRLoginType) (*QRCode, error) {
	if loginType == QRLoginWX {
		return c.getWXQRCode()
	}
	return c.getQQQRCode()
}

// getQQQRCode 获取 QQ 授权二维码。
func (c *Client) getQQQRCode() (*QRCode, error) {
	l := newLoginHTTP()
	url := "https://ssl.ptlogin2.qq.com/ptqrshow" +
		"?appid=716027609&e=2&l=M&s=3&d=72&v=4&t=0.123456789&daid=383&pt_3rd_aid=100497308"

	resp, body, err := l.get(url, map[string]string{
		"Referer": "https://xui.ptlogin2.qq.com/",
	})
	if err != nil {
		return nil, fmt.Errorf("qqmusic: 获取 QQ 二维码失败: %w", err)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("qqmusic: QQ 二维码响应为空")
	}

	// qrsig 在 Set-Cookie 中，是后续轮询的凭据。
	qrsig := ""
	for _, ck := range resp.Cookies() {
		if ck.Name == "qrsig" {
			qrsig = ck.Value
			break
		}
	}
	if qrsig == "" {
		return nil, fmt.Errorf("qqmusic: 未获取到 qrsig")
	}

	return &QRCode{
		Image:      body,
		MimeType:   "image/png",
		Identifier: qrsig,
		Type:       QRLoginQQ,
	}, nil
}

// getWXQRCode 获取微信登录二维码。
func (c *Client) getWXQRCode() (*QRCode, error) {
	l := newLoginHTTP()
	url := "https://open.weixin.qq.com/connect/qrconnect" +
		"?appid=wx48db31d50e334801" +
		"&redirect_uri=https%3A%2F%2Fy.qq.com%2Fportal%2Fwx_redirect.html%3Flogin_type%3D2%26surl%3Dhttps%3A%2F%2Fy.qq.com%2F" +
		"&response_type=code&scope=snsapi_login&state=STATE"

	_, body, err := l.get(url, nil)
	if err != nil {
		return nil, fmt.Errorf("qqmusic: 获取微信二维码页失败: %w", err)
	}

	matches := wxUUIDRe.FindSubmatch(body)
	if len(matches) < 2 {
		return nil, fmt.Errorf("qqmusic: 未获取到微信 uuid")
	}
	uuid := string(matches[1])

	_, img, err := l.get("https://open.weixin.qq.com/connect/qrcode/"+uuid, map[string]string{
		"Referer": "https://open.weixin.qq.com/connect/qrconnect",
	})
	if err != nil {
		return nil, fmt.Errorf("qqmusic: 获取微信二维码图片失败: %w", err)
	}

	return &QRCode{
		Image:      img,
		MimeType:   "image/jpeg",
		Identifier: uuid,
		Type:       QRLoginWX,
	}, nil
}

// CheckQRCode 轮询扫码状态。
func (c *Client) CheckQRCode(qr *QRCode) QRLoginResult {
	if qr.Type == QRLoginWX {
		return c.checkWXQRCode(qr)
	}
	return c.checkQQQRCode(qr)
}

// checkQQQRCode 检查 QQ 二维码状态。
func (c *Client) checkQQQRCode(qr *QRCode) QRLoginResult {
	l := newLoginHTTP()

	ptqrtoken := ptqrToken(qr.Identifier)
	url := fmt.Sprintf(
		"https://ssl.ptlogin2.qq.com/ptqrlogin"+
			"?u1=https%%3A%%2F%%2Fgraph.qq.com%%2Foauth2.0%%2Flogin_jump"+
			"&ptqrtoken=%d&ptredirect=0&h=1&t=1&g=1&from_ui=1&ptlang=2052"+
			"&action=0-0-%d&js_ver=20102616&js_type=1&pt_uistyle=40"+
			"&aid=716027609&daid=383&pt_3rd_aid=100497308&has_onekey=1",
		ptqrtoken, time.Now().UnixMilli(),
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return QRLoginResult{Err: err}
	}
	req.Header.Set("Referer", "https://xui.ptlogin2.qq.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.AddCookie(&http.Cookie{Name: "qrsig", Value: qr.Identifier})

	resp, err := l.client.Do(req)
	if err != nil {
		return QRLoginResult{Err: fmt.Errorf("qqmusic: 轮询 QQ 扫码状态失败: %w", err)}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return QRLoginResult{Err: err}
	}

	statusMatch := qqStatusRe.FindSubmatch(body)
	if len(statusMatch) < 2 {
		return QRLoginResult{Err: fmt.Errorf("qqmusic: 无法解析扫码状态")}
	}

	args := qqArgsRe.FindAllSubmatch(statusMatch[1], -1)
	if len(args) == 0 {
		return QRLoginResult{Err: fmt.Errorf("qqmusic: 扫码状态参数缺失")}
	}

	var code int
	if _, err := fmt.Sscanf(string(args[0][1]), "%d", &code); err != nil {
		return QRLoginResult{Err: fmt.Errorf("qqmusic: 扫码状态码无效")}
	}

	// QQ 状态码：66=等待扫码，67=已扫码待确认，65=已过期，0=成功，1=拒绝
	switch code {
	case 66:
		return QRLoginResult{Event: QREventWaiting}
	case 67:
		return QRLoginResult{Event: QREventScanned}
	case 65:
		return QRLoginResult{Event: QREventExpired}
	case 1:
		return QRLoginResult{Event: QREventRefused}
	case 0:
		// 成功：args[2] 是跳转 URL，含 uin 与 ptsigx
	default:
		return QRLoginResult{Event: QREventWaiting}
	}

	if len(args) < 3 {
		return QRLoginResult{Err: fmt.Errorf("qqmusic: 登录凭据参数缺失")}
	}
	redirect := string(args[2][1])

	sigxMatch := qqSigxRe.FindStringSubmatch(redirect)
	uinMatch := qqUinRe.FindStringSubmatch(redirect)
	if len(sigxMatch) < 2 || len(uinMatch) < 2 {
		return QRLoginResult{Err: fmt.Errorf("qqmusic: 无法解析 uin/ptsigx")}
	}

	cred, err := c.authorizeQQQR(uinMatch[1], sigxMatch[1])
	if err != nil {
		return QRLoginResult{Err: err}
	}
	return QRLoginResult{Event: QREventDone, Credential: cred}
}

// authorizeQQQR 完成 QQ 二维码鉴权。
//
// 流程对齐 QQMusicApi _authorize_qq_qr：
//  1. check_sig 换取 p_skey（不跟随重定向，从 Set-Cookie 读取）
//  2. 用 p_skey 计算 g_tk，POST oauth2.0/authorize 换取 code
//  3. 用 code 调用 QQConnectLogin.LoginServer.QQLogin 换凭证
func (c *Client) authorizeQQQR(uin, sigx string) (*Credential, error) {
	// 整个鉴权链路必须共用同一个 CookieJar：p_skey 由第 1 步种下，
	// 第 2 步要带上它，否则拿不到 code。
	l := newLoginHTTP()

	checkSigURL := "https://ssl.ptlogin2.graph.qq.com/check_sig" +
		"?uin=" + uin +
		"&pttype=1&service=ptqrlogin&nodirect=0" +
		"&ptsigx=" + sigx +
		"&s_url=https%3A%2F%2Fgraph.qq.com%2Foauth2.0%2Flogin_jump" +
		"&ptlang=2052&ptredirect=100&aid=716027609&daid=383" +
		"&j_later=0&low_login_hour=0&regmaster=0&pt_login_type=3" +
		"&pt_aid=0&pt_aaid=16&pt_light=0&pt_3rd_aid=100497308"

	resp, _, err := l.get(checkSigURL, map[string]string{
		"Referer": "https://xui.ptlogin2.qq.com/",
	})
	if err != nil {
		return nil, fmt.Errorf("qqmusic: check_sig 失败: %w", err)
	}

	pSkey := ""
	for _, ck := range resp.Cookies() {
		if ck.Name == "p_skey" {
			pSkey = ck.Value
			break
		}
	}
	if pSkey == "" {
		return nil, fmt.Errorf("qqmusic: 未获取到 p_skey")
	}

	// 注意：参考实现用 POST 提交表单，且 scope 含 get_app_friends。
	form := url.Values{}
	form.Set("response_type", "code")
	form.Set("client_id", "100497308")
	form.Set("redirect_uri", "https://y.qq.com/portal/wx_redirect.html?login_type=1&surl=https://y.qq.com/")
	form.Set("scope", "get_user_info,get_app_friends")
	form.Set("state", "state")
	form.Set("switch", "")
	form.Set("from_ptlogin", "1")
	form.Set("src", "1")
	form.Set("update_auth", "1")
	form.Set("openapi", "1010_1030")
	form.Set("g_tk", strconv.Itoa(gtkFromSkey(pSkey)))
	form.Set("auth_time", strconv.FormatInt(time.Now().UnixMilli(), 10))
	form.Set("ui", newUUID())

	authResp, _, err := l.postForm("https://graph.qq.com/oauth2.0/authorize", form, map[string]string{
		"Referer": "https://graph.qq.com/",
	})
	if err != nil {
		return nil, fmt.Errorf("qqmusic: oauth authorize 失败: %w", err)
	}

	code := extractQueryParam(authResp.Header.Get("Location"), "code")
	if code == "" {
		return nil, fmt.Errorf("qqmusic: 未获取到授权 code")
	}

	return c.loginByQQCode(code)
}

// extractQueryParam 从 URL 中提取查询参数值。
//
// Go 的 regexp 不支持 lookbehind，因此用 net/url 解析而不是正则。
func extractQueryParam(rawURL, key string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Query().Get(key)
}

// newUUID 生成一个随机 UUID v4 字符串，用于 authorize 的 ui 参数。
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return randHex(32)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// loginByQQCode 用 QQ 授权 code 换取音乐登录态。
//
// 必须使用 QQConnectLogin.LoginServer.QQLogin，而不是微信登录用的
// music.login.LoginServer.Login。用错模块会返回一份「看似成功」但
// 并非有效 QQ 登录态的凭证：它能通过 LoginServer 刷新，但 vkey
// 会以 retcode=104009 (invalidq) 拒绝，表现为无法播放。
func (c *Client) loginByQQCode(code string) (*Credential, error) {
	data, err := c.Do(cgiRequest{
		Module: "QQConnectLogin.LoginServer",
		Method: "QQLogin",
		Param:  map[string]any{"code": code},
		Comm:   map[string]any{"tmeLoginType": 2},
	})
	if err != nil {
		return nil, err
	}
	return parseCredential(data)
}

// checkWXQRCode 检查微信二维码状态。
func (c *Client) checkWXQRCode(qr *QRCode) QRLoginResult {
	l := newLoginHTTP()

	url := fmt.Sprintf(
		"https://lp.open.weixin.qq.com/connect/l/qrconnect?uuid=%s&_=%d",
		qr.Identifier, time.Now().UnixMilli(),
	)

	_, body, err := l.get(url, map[string]string{
		"Referer": "https://open.weixin.qq.com/",
	})
	if err != nil {
		// 长轮询超时视为「等待扫码」，不当作错误。
		return QRLoginResult{Event: QREventWaiting}
	}

	matches := wxStatRe.FindSubmatch(body)
	if len(matches) < 3 {
		return QRLoginResult{Event: QREventWaiting}
	}

	var errcode int
	if _, err := fmt.Sscanf(string(matches[1]), "%d", &errcode); err != nil {
		return QRLoginResult{Err: fmt.Errorf("qqmusic: 微信状态码无效")}
	}

	// 微信状态码：408=等待，404=已扫码，405=已确认，403=拒绝，402=过期，0=成功
	switch errcode {
	case 408:
		return QRLoginResult{Event: QREventWaiting}
	case 404:
		return QRLoginResult{Event: QREventScanned}
	case 405:
		return QRLoginResult{Event: QREventConfirmed}
	case 403:
		return QRLoginResult{Event: QREventRefused}
	case 402:
		return QRLoginResult{Event: QREventExpired}
	case 0:
		wxCode := string(matches[2])
		if wxCode == "" {
			return QRLoginResult{Err: fmt.Errorf("qqmusic: 微信 code 为空")}
		}
		cred, err := c.loginByWXCode(wxCode)
		if err != nil {
			return QRLoginResult{Err: err}
		}
		return QRLoginResult{Event: QREventDone, Credential: cred}
	default:
		return QRLoginResult{Event: QREventWaiting}
	}
}

// loginByWXCode 用微信授权 code 换取音乐登录态。
func (c *Client) loginByWXCode(code string) (*Credential, error) {
	data, err := c.doLogin(
		map[string]any{"code": code, "strAppid": "wx48db31d50e334801"},
		map[string]any{"tmeLoginType": 1},
	)
	if err != nil {
		return nil, err
	}
	return parseCredential(data)
}

// doLogin 调用 LoginServer.Login 完成登录。
func (c *Client) doLogin(param map[string]any, comm map[string]any) (json.RawMessage, error) {
	return c.Do(cgiRequest{
		Module: "music.login.LoginServer",
		Method: "Login",
		Param:  param,
		Comm:   comm,
	})
}

// Refresh 用现有凭证换取新的 musickey。
//
// QQ 的 musickey 是短时效的（数小时到一天），过期后所有需要登录的接口
// 都会退化为匿名，表现为 vkey 返回空 purl、result=104003。
// 扫码登录拿到的 key 也可能在写入存储前就已过期，因此启动时必须先刷新。
//
// 刷新成功会更新内存与返回值；失败保留原凭证。
func (c *Client) Refresh() (*Credential, error) {
	old := c.Credential()
	if !old.Valid() {
		return nil, ErrNotLoggedIn
	}

	// 刷新请求本身要用旧 key 做鉴权，因此 comm 里带旧 authst。
	data, err := c.Do(cgiRequest{
		Module: "music.login.LoginServer",
		Method: "Login",
		Param: map[string]any{
			"musicid":  old.MusicID,
			"musickey": old.MusicKey,
		},
		Comm: map[string]any{
			"ct":           "11",
			"cv":           "20090008",
			"v":            "20090008",
			"chid":         "10003505",
			"tmeAppID":     "qqmusic",
			"qq":           old.Uin(),
			"authst":       old.MusicKey,
			"tmeLoginType": strconv.Itoa(max(old.LoginTyp, 1)),
			"format":       "json",
			"inCharset":    "utf-8",
			"outCharset":   "utf-8",
		},
	})
	if err != nil {
		return nil, err
	}

	cred, err := parseCredential(data)
	if err != nil {
		return nil, err
	}
	// 刷新响应可能不带 openid/unionid，沿用旧值。
	if cred.OpenID == "" {
		cred.OpenID = old.OpenID
	}
	if cred.UnionID == "" {
		cred.UnionID = old.UnionID
	}
	if cred.EncryptU == "" {
		cred.EncryptU = old.EncryptU
	}
	if cred.LoginTyp == 0 {
		cred.LoginTyp = old.LoginTyp
	}

	c.SetCredential(*cred)
	return cred, nil
}

// EnsureFresh 在凭证即将过期时刷新一次。
//
// 只在已登录且有效期不足 1 小时（或已过期）时触发，避免每次启动
// 都打一次刷新请求。刷新失败不阻断调用方，由后续接口自行报错。
func (c *Client) EnsureFresh() {
	cred := c.Credential()
	if !cred.Valid() {
		return
	}
	// 缺少有效期信息时无法判断，保守刷新一次。
	if cred.KeyExpiresIn > 0 && cred.KeyCreateAt > 0 {
		const refreshAhead = 3600
		if time.Now().Unix() < cred.KeyCreateAt+int64(cred.KeyExpiresIn)-refreshAhead {
			return
		}
	}
	if _, err := c.Refresh(); err != nil {
		slog.Warn("qqmusic: 刷新登录态失败", "error", err)
	}
}

// parseCredential 从登录响应中解析凭证。
func parseCredential(data json.RawMessage) (*Credential, error) {
	var parsed struct {
		Code         int    `json:"code"`
		MusicID      int64  `json:"musicid"`
		MusicKey     string `json:"musickey"`
		OpenID       string `json:"openid"`
		UnionID      string `json:"unionid"`
		Encrypt      string `json:"encryptUin"`
		LoginTyp     int    `json:"loginType"`
		StrID        string `json:"str_musicid"`
		KeyExpiresIn int    `json:"keyExpiresIn"`
		KeyCreateAt  int64  `json:"musickeyCreateTime"`
	}
	if err := decode(data, &parsed); err != nil {
		return nil, err
	}
	if parsed.Code != 0 {
		return nil, &APIError{Code: parsed.Code, Msg: "登录失败"}
	}
	if parsed.MusicID == 0 || parsed.MusicKey == "" {
		return nil, fmt.Errorf("qqmusic: 登录响应缺少 musicid/musickey")
	}

	cred := &Credential{
		MusicID:      parsed.MusicID,
		StrMusicID:   parsed.StrID,
		MusicKey:     parsed.MusicKey,
		OpenID:       parsed.OpenID,
		UnionID:      parsed.UnionID,
		EncryptU:     parsed.Encrypt,
		LoginTyp:     parsed.LoginTyp,
		KeyExpiresIn: parsed.KeyExpiresIn,
		KeyCreateAt:  parsed.KeyCreateAt,
	}
	return cred, nil
}

// hash33 计算 Hash33 哈希，用于 ptqrtoken 与 g_tk。
//
// 注意：这两个用途的初值不同，混用会导致服务端返回 403。
//   - ptqrtoken（扫码轮询）初值为 0
//   - g_tk（OAuth 鉴权）初值为 5381
func hash33(s string, h int) int {
	for _, ch := range s {
		h = (h << 5) + h + int(ch)
	}
	return 2147483647 & h
}

// ptqrToken 计算扫码轮询所需的 ptqrtoken（初值 0）。
func ptqrToken(qrsig string) int {
	return hash33(qrsig, 0)
}

// gtkFromSkey 计算 g_tk（初值 5381）。
func gtkFromSkey(skey string) int {
	return hash33(skey, 5381)
}

// LoginByCredential 校验并应用凭证。
func (c *Client) LoginByCredential(cred Credential) error {
	if !cred.Valid() {
		return ErrNotLoggedIn
	}
	c.SetCredential(cred)
	return nil
}

// Logout 清除本地登录态。
func (c *Client) Logout() {
	c.SetCredential(Credential{})
}
