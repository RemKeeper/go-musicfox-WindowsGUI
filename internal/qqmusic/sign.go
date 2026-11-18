package qqmusic

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
)

// 签名分片索引（移植自 QQMusicApi algorithms/sign.py）。
var (
	signPart1Indexes = []int{23, 14, 6, 36, 16, 7, 19}
	signPart2Indexes = []int{16, 1, 32, 12, 19, 27, 8, 5}
	signScramble     = []byte{89, 39, 179, 150, 218, 82, 58, 252, 177, 52, 186, 123, 120, 64, 242, 133, 143, 161, 121, 179}
)

// Sign 计算 QQ 音乐客户端请求的 zzc 签名。
//
// payload 必须与最终发送的请求体逐字节一致，否则服务端拒绝。
func Sign(payload []byte) string {
	sum := sha1.Sum(payload)
	h := strings.ToUpper(hex.EncodeToString(sum[:]))

	var part1 strings.Builder
	part1.Grow(len(signPart1Indexes))
	for _, i := range signPart1Indexes {
		part1.WriteByte(h[i])
	}

	var part2 strings.Builder
	part2.Grow(len(signPart2Indexes))
	for _, i := range signPart2Indexes {
		part2.WriteByte(h[i])
	}

	part3 := make([]byte, 20)
	for i, v := range signScramble {
		// h 始终是合法十六进制，解析不会失败。
		b, _ := strconv.ParseUint(h[i*2:i*2+2], 16, 8)
		part3[i] = v ^ byte(b)
	}

	// 去掉 base64 中的 \ / + = 四个字符
	b64 := base64.StdEncoding.EncodeToString(part3)
	b64 = strings.NewReplacer(`\`, "", "/", "", "+", "", "=", "").Replace(b64)

	return strings.ToLower("zzc" + part1.String() + b64 + part2.String())
}
