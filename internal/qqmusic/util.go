package qqmusic

import (
	"math/rand/v2"
	"time"
)

// randIntN 是 rand.IntN 的薄封装，集中随机源便于测试替换。
func randIntN(n int) int {
	if n <= 0 {
		return 0
	}
	return rand.IntN(n)
}

// timeNowMillis 返回当前毫秒时间戳。
func timeNowMillis() int64 {
	return time.Now().UnixMilli()
}
