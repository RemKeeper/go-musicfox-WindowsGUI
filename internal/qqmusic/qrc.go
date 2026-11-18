package qqmusic

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// qrc3DESKey 是 QQ 音乐 QRC 歌词解密使用的 24 字节密钥。
var qrc3DESKey = []byte("!@#)(*$%123ZXC!@!@#)(NHL")

// 3DES 模式。
const (
	desEncrypt = 1
	desDecrypt = 0
)

// sboxBit 对输入整数重新组合位。
func sboxBit(a int) int {
	return (a & 32) | ((a & 31) >> 1) | ((a & 1) << 4)
}

// initialPermutation 执行初始置换，返回左右两个 32 位整数。
func initialPermutation(in []byte) (uint32, uint32) {
	v0 := uint32(in[0]) | uint32(in[1])<<8 | uint32(in[2])<<16 | uint32(in[3])<<24
	v1 := uint32(in[4]) | uint32(in[5])<<8 | uint32(in[6])<<16 | uint32(in[7])<<24

	bit := func(v uint32, shift uint) uint32 { return (v >> shift) & 1 }

	s0 := bit(v1, 6)<<31 | bit(v1, 14)<<30 | bit(v1, 22)<<29 | bit(v1, 30)<<28 |
		bit(v0, 6)<<27 | bit(v0, 14)<<26 | bit(v0, 22)<<25 | bit(v0, 30)<<24 |
		bit(v1, 4)<<23 | bit(v1, 12)<<22 | bit(v1, 20)<<21 | bit(v1, 28)<<20 |
		bit(v0, 4)<<19 | bit(v0, 12)<<18 | bit(v0, 20)<<17 | bit(v0, 28)<<16 |
		bit(v1, 2)<<15 | bit(v1, 10)<<14 | bit(v1, 18)<<13 | bit(v1, 26)<<12 |
		bit(v0, 2)<<11 | bit(v0, 10)<<10 | bit(v0, 18)<<9 | bit(v0, 26)<<8 |
		bit(v1, 0)<<7 | bit(v1, 8)<<6 | bit(v1, 16)<<5 | bit(v1, 24)<<4 |
		bit(v0, 0)<<3 | bit(v0, 8)<<2 | bit(v0, 16)<<1 | bit(v0, 24)

	s1 := bit(v1, 7)<<31 | bit(v1, 15)<<30 | bit(v1, 23)<<29 | bit(v1, 31)<<28 |
		bit(v0, 7)<<27 | bit(v0, 15)<<26 | bit(v0, 23)<<25 | bit(v0, 31)<<24 |
		bit(v1, 5)<<23 | bit(v1, 13)<<22 | bit(v1, 21)<<21 | bit(v1, 29)<<20 |
		bit(v0, 5)<<19 | bit(v0, 13)<<18 | bit(v0, 21)<<17 | bit(v0, 29)<<16 |
		bit(v1, 3)<<15 | bit(v1, 11)<<14 | bit(v1, 19)<<13 | bit(v1, 27)<<12 |
		bit(v0, 3)<<11 | bit(v0, 11)<<10 | bit(v0, 19)<<9 | bit(v0, 27)<<8 |
		bit(v1, 1)<<7 | bit(v1, 9)<<6 | bit(v1, 17)<<5 | bit(v1, 25)<<4 |
		bit(v0, 1)<<3 | bit(v0, 9)<<2 | bit(v0, 17)<<1 | bit(v0, 25)

	return s0, s1
}

// inversePermutation 执行逆初始置换，返回 8 字节。
func inversePermutation(s0, s1 uint32) []byte {
	out := make([]byte, 8)
	bit := func(v uint32, shift uint) byte { return byte((v >> shift) & 1) }

	out[3] = bit(s1, 24)<<7 | bit(s0, 24)<<6 | bit(s1, 16)<<5 | bit(s0, 16)<<4 |
		bit(s1, 8)<<3 | bit(s0, 8)<<2 | bit(s1, 0)<<1 | bit(s0, 0)
	out[2] = bit(s1, 25)<<7 | bit(s0, 25)<<6 | bit(s1, 17)<<5 | bit(s0, 17)<<4 |
		bit(s1, 9)<<3 | bit(s0, 9)<<2 | bit(s1, 1)<<1 | bit(s0, 1)
	out[1] = bit(s1, 26)<<7 | bit(s0, 26)<<6 | bit(s1, 18)<<5 | bit(s0, 18)<<4 |
		bit(s1, 10)<<3 | bit(s0, 10)<<2 | bit(s1, 2)<<1 | bit(s0, 2)
	out[0] = bit(s1, 27)<<7 | bit(s0, 27)<<6 | bit(s1, 19)<<5 | bit(s0, 19)<<4 |
		bit(s1, 11)<<3 | bit(s0, 11)<<2 | bit(s1, 3)<<1 | bit(s0, 3)
	out[7] = bit(s1, 28)<<7 | bit(s0, 28)<<6 | bit(s1, 20)<<5 | bit(s0, 20)<<4 |
		bit(s1, 12)<<3 | bit(s0, 12)<<2 | bit(s1, 4)<<1 | bit(s0, 4)
	out[6] = bit(s1, 29)<<7 | bit(s0, 29)<<6 | bit(s1, 21)<<5 | bit(s0, 21)<<4 |
		bit(s1, 13)<<3 | bit(s0, 13)<<2 | bit(s1, 5)<<1 | bit(s0, 5)
	out[5] = bit(s1, 30)<<7 | bit(s0, 30)<<6 | bit(s1, 22)<<5 | bit(s0, 22)<<4 |
		bit(s1, 14)<<3 | bit(s0, 14)<<2 | bit(s1, 6)<<1 | bit(s0, 6)
	out[4] = bit(s1, 31)<<7 | bit(s0, 31)<<6 | bit(s1, 23)<<5 | bit(s0, 23)<<4 |
		bit(s1, 15)<<3 | bit(s0, 15)<<2 | bit(s1, 7)<<1 | bit(s0, 7)
	return out
}

// desF 是 DES 的 F 函数。
func desF(state uint32, key []byte) uint32 {
	t1 := ((state & 1) << 31) |
		((state & 0xF8000000) >> 1) |
		((state & 0x1F800000) >> 3) |
		((state & 0x01F80000) >> 5) |
		((state & 0x001F8000) >> 7)
	t2 := ((state & 0x0001F800) << 15) |
		((state & 0x00001F80) << 13) |
		((state & 0x000001F8) << 11) |
		((state & 0x0000001F) << 9) |
		((state & 0x80000000) >> 23)

	k0 := byte((t1>>24)&0xFF) ^ key[0]
	k1 := byte((t1>>16)&0xFF) ^ key[1]
	k2 := byte((t1>>8)&0xFF) ^ key[2]
	k3 := byte((t2>>24)&0xFF) ^ key[3]
	k4 := byte((t2>>16)&0xFF) ^ key[4]
	k5 := byte((t2>>8)&0xFF) ^ key[5]

	s := uint32(sbox[0][sboxBit(int(k0>>2))])<<28 |
		uint32(sbox[1][sboxBit(int((k0&0x03)<<4|(k1>>4)))])<<24 |
		uint32(sbox[2][sboxBit(int((k1&0x0F)<<2|(k2>>6)))])<<20 |
		uint32(sbox[3][sboxBit(int(k2&0x3F))])<<16 |
		uint32(sbox[4][sboxBit(int(k3>>2))])<<12 |
		uint32(sbox[5][sboxBit(int((k3&0x03)<<4|(k4>>4)))])<<8 |
		uint32(sbox[6][sboxBit(int((k4&0x0F)<<2|(k5>>6)))])<<4 |
		uint32(sbox[7][sboxBit(int(k5&0x3F))])

	bit := func(shift uint) uint32 { return (s >> shift) & 1 }

	return bit(16)<<31 | bit(25)<<30 | bit(12)<<29 | bit(11)<<28 |
		bit(3)<<27 | bit(20)<<26 | bit(4)<<25 | bit(15)<<24 |
		bit(31)<<23 | bit(17)<<22 | bit(9)<<21 | bit(6)<<20 |
		bit(27)<<19 | bit(14)<<18 | bit(1)<<17 | bit(22)<<16 |
		bit(30)<<15 | bit(24)<<14 | bit(8)<<13 | bit(18)<<12 |
		bit(0)<<11 | bit(5)<<10 | bit(29)<<9 | bit(23)<<8 |
		bit(13)<<7 | bit(19)<<6 | bit(2)<<5 | bit(26)<<4 |
		bit(10)<<3 | bit(21)<<2 | bit(28)<<1 | bit(7)
}

// desCrypt 执行单次 DES 块运算。
func desCrypt(in []byte, key [][]byte) []byte {
	s0, s1 := initialPermutation(in)
	for i := 0; i < 15; i++ {
		prevS1 := s1
		s1 = desF(s1, key[i]) ^ s0
		s0 = prevS1
	}
	s0 = desF(s1, key[15]) ^ s0
	return inversePermutation(s0, s1)
}

var (
	keyRndShift    = []int{1, 1, 2, 2, 2, 2, 2, 2, 1, 2, 2, 2, 2, 2, 2, 1}
	keyPermC       = []int{56, 48, 40, 32, 24, 16, 8, 0, 57, 49, 41, 33, 25, 17, 9, 1, 58, 50, 42, 34, 26, 18, 10, 2, 59, 51, 43, 35}
	keyPermD       = []int{62, 54, 46, 38, 30, 22, 14, 6, 61, 53, 45, 37, 29, 21, 13, 5, 60, 52, 44, 36, 28, 20, 12, 4, 27, 19, 11, 3}
	keyCompression = []int{
		13, 16, 10, 23, 0, 4, 2, 27, 14, 5, 20, 9, 22, 18, 11, 3,
		25, 7, 15, 6, 26, 19, 12, 1, 40, 51, 30, 36, 46, 54, 29, 39,
		50, 44, 32, 47, 43, 48, 38, 55, 33, 52, 45, 41, 49, 35, 28, 31,
	}
)

// keySchedule 是 DES 密钥扩展（含 QQ 音乐自定义的 PC-2 偏移）。
func keySchedule(key []byte, mode int) [][]byte {
	schedule := make([][]byte, 16)
	for i := range schedule {
		schedule[i] = make([]byte, 6)
	}

	v0 := uint32(key[0]) | uint32(key[1])<<8 | uint32(key[2])<<16 | uint32(key[3])<<24
	v1 := uint32(key[4]) | uint32(key[5])<<8 | uint32(key[6])<<16 | uint32(key[7])<<24

	pick := func(b int) uint32 {
		if b < 32 {
			return (v0 >> (31 - uint(b))) & 1
		}
		return (v1 >> (63 - uint(b))) & 1
	}

	var c, d uint32
	for i, b := range keyPermC {
		c |= pick(b) << (31 - uint(i))
	}
	for i, b := range keyPermD {
		d |= pick(b) << (31 - uint(i))
	}

	for i := 0; i < 16; i++ {
		sh := uint(keyRndShift[i])
		c = ((c << sh) | (c >> (28 - sh))) & 0xFFFFFFF0
		d = ((d << sh) | (d >> (28 - sh))) & 0xFFFFFFF0

		togen := i
		if mode == desDecrypt {
			togen = 15 - i
		}

		for j := 0; j < 6; j++ {
			schedule[togen][j] = 0
		}
		for j := 0; j < 24; j++ {
			b := (c >> (31 - uint(keyCompression[j]))) & 1
			schedule[togen][j/8] |= byte(b) << (7 - uint(j%8))
		}
		for j := 24; j < 48; j++ {
			b := (d >> (31 - uint(keyCompression[j]-27))) & 1
			schedule[togen][j/8] |= byte(b) << (7 - uint(j%8))
		}
	}
	return schedule
}

// tripleDESKeySetup 生成 3DES 的三组子密钥。
func tripleDESKeySetup(key []byte, mode int) [][][]byte {
	if mode == desEncrypt {
		return [][][]byte{
			keySchedule(key[0:8], desEncrypt),
			keySchedule(key[8:16], desDecrypt),
			keySchedule(key[16:24], desEncrypt),
		}
	}
	return [][][]byte{
		keySchedule(key[16:24], desDecrypt),
		keySchedule(key[8:16], desEncrypt),
		keySchedule(key[0:8], desDecrypt),
	}
}

// tripleDESCrypt 对单个 8 字节块执行 3DES。
func tripleDESCrypt(data []byte, key [][][]byte) []byte {
	cur := data
	for i := 0; i < 3; i++ {
		cur = desCrypt(cur, key[i])
	}
	return cur
}

// QrcDecrypt 解密 QQ 音乐 QRC 歌词。
//
// 输入为接口返回的十六进制字符串，输出为解密后的 XML 文本。
func QrcDecrypt(encrypted string) (string, error) {
	if encrypted == "" {
		return "", nil
	}

	raw, err := hex.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("qrc: 十六进制解码失败: %w", err)
	}
	if len(raw)%8 != 0 {
		return "", errors.New("qrc: 密文长度不是 8 的倍数")
	}

	schedule := tripleDESKeySetup(qrc3DESKey, desDecrypt)
	plain := make([]byte, 0, len(raw))
	for i := 0; i < len(raw); i += 8 {
		plain = append(plain, tripleDESCrypt(raw[i:i+8], schedule)...)
	}

	zr, err := zlib.NewReader(bytes.NewReader(plain))
	if err != nil {
		return "", fmt.Errorf("qrc: zlib 初始化失败: %w", err)
	}
	defer zr.Close()

	out, err := io.ReadAll(zr)
	if err != nil {
		return "", fmt.Errorf("qrc: zlib 解压失败: %w", err)
	}
	return string(out), nil
}

// QrcLyricContent 从 QRC XML 中提取 LyricContent 属性并反转义。
//
// QRC 解密结果是形如：
//
//	<QrcInfos>...<Lyric_1 LyricType="1" LyricContent="[ti:...]&#10;[0,4503]词(0,450)..." />
//
// 返回的属性值已反转义，可直接交给 lyric.ParseYRC 解析。
func QrcLyricContent(xml string) string {
	const marker = `LyricContent="`
	idx := strings.Index(xml, marker)
	if idx < 0 {
		return ""
	}
	rest := xml[idx+len(marker):]

	// 属性值以未转义的 `"` 结束；`&quot;` 是歌词内的引号，不是终止符。
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return unescapeXMLAttr(rest[:end])
}

// unescapeXMLAttr 反转义 XML 属性值中的实体。
func unescapeXMLAttr(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	replacer := strings.NewReplacer(
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&apos;", "'",
		"&#10;", "\n",
		"&#13;", "\r",
		"&#9;", "\t",
		"&amp;", "&", // 必须最后处理
	)
	return replacer.Replace(s)
}

// qrcToYRC 将 QRC 的逐字歌词转换为 musicfox YRC 解析器可消费的格式。
//
// QRC 词元为 `词(起始ms,时长ms)`，而 internal/lyric 的 LRC 风格 YRC 解析器接受
// `词(起始ms,时长ms,0)` 或 `词(起始ms,时长ms)`（判定为 len(parts) < 2 才跳过），
// 因此两者天然兼容，这里只需剥离 XML 外壳。
func qrcToYRC(xml string) string {
	return QrcLyricContent(xml)
}

// formatInt 是 strconv.Itoa 的薄封装，便于在拼接中保持可读性。
func formatInt(v int) string { return strconv.Itoa(v) }
