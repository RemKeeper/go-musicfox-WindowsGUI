package qqmusic

import (
	"testing"
)

// TestIntegrationQRLoginPoll 验证扫码轮询能拿到有效响应。
//
// 这是「一直等待扫码」bug 的端到端回归测试：
// 修复前 ptqrtoken 用错初值（5381），服务端返回 403，轮询永远失败，
// 界面卡在「等待扫码」。修复后应返回 QREventWaiting（code=66）。
func TestIntegrationQRLoginPoll(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要外网的测试")
	}

	c := NewClient()

	qr, err := c.GetQRCode(QRLoginQQ)
	if err != nil {
		t.Fatalf("GetQRCode 失败: %v", err)
	}
	if len(qr.Image) == 0 {
		t.Fatal("二维码图片为空")
	}
	if qr.Identifier == "" {
		t.Fatal("qrsig 为空")
	}
	t.Logf("二维码 %d 字节, qrsig 长度 %d, ptqrtoken=%d",
		len(qr.Image), len(qr.Identifier), ptqrToken(qr.Identifier))

	// 首次轮询：未扫码时应返回 Waiting，而不是错误。
	result := c.CheckQRCode(qr)
	if result.Err != nil {
		t.Fatalf("轮询返回错误（说明 403 未修复）: %v", result.Err)
	}
	if result.Event != QREventWaiting {
		t.Errorf("未扫码时事件 = %v, want QREventWaiting", result.Event)
	}
	t.Logf("轮询正常，事件 = %v", result.Event)

	// 连续轮询几次，确认稳定（不出现间歇性 403）。
	for i := 0; i < 3; i++ {
		r := c.CheckQRCode(qr)
		if r.Err != nil {
			t.Fatalf("第 %d 次轮询失败: %v", i+2, r.Err)
		}
		if r.Event != QREventWaiting {
			t.Errorf("第 %d 次轮询事件 = %v, want QREventWaiting", i+2, r.Event)
		}
	}
	t.Log("连续 4 次轮询均正常")
}

// TestIntegrationWXQRCode 验证微信二维码可获取。
func TestIntegrationWXQRCode(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要外网的测试")
	}

	c := NewClient()
	qr, err := c.GetQRCode(QRLoginWX)
	if err != nil {
		t.Fatalf("获取微信二维码失败: %v", err)
	}
	if len(qr.Image) == 0 {
		t.Fatal("微信二维码图片为空")
	}
	if qr.Identifier == "" {
		t.Fatal("微信 uuid 为空")
	}
	t.Logf("微信二维码 %d 字节, uuid=%s", len(qr.Image), qr.Identifier)
}
