package app

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeGradientImage 构造一张非标准尺寸的灰度渐变图，用于触发退化渲染路径。
func makeGradientImage(w, h int) image.Image {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8((x*255 + y*255) / (w + h))
			img.SetGray(x, y, color.Gray{Y: v})
		}
	}
	return img
}

// loadQRFixture 读取真实二维码图片夹具（QQ 扫码登录接口返回的 PNG）。
func loadQRFixture(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join("testdata", "qq_qrcode.png")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取二维码夹具失败: %v", err)
	}
	return data
}

// TestGenQRCodeFromImageFitsTerminal 是二维码尺寸的回归测试。
//
// 历史 bug：QQ 二维码是 111x111 像素（33 模块 × 3 像素/模块 + 静区），
// 逐像素渲染得到 111 列 × 56 行，远超终端宽度，用户看不到完整二维码。
//
// 修复后应先还原 33×33 模块矩阵，再按 2 模块行/字符行合成，
// 输出 33 列 × 17 行。
func TestGenQRCodeFromImageFitsTerminal(t *testing.T) {
	img := loadQRFixture(t)

	_, buf, err := GenQRCodeFromImage("test-qr.png", img)
	if err != nil {
		t.Fatalf("GenQRCodeFromImage 失败: %v", err)
	}

	view := buf.String()
	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	if len(lines) == 0 {
		t.Fatal("渲染结果为空")
	}

	cols := len([]rune(lines[0]))
	rows := len(lines)

	t.Logf("渲染尺寸: %d 列 × %d 行", cols, rows)

	// 33 模块 → 33 列；33 模块行 → 17 字符行。
	if cols > 60 {
		t.Errorf("宽度 %d 列超出终端可显示范围（应 ≤ 60）", cols)
	}
	if rows > 30 {
		t.Errorf("高度 %d 行超出终端可显示范围（应 ≤ 30）", rows)
	}

	// 每行列数必须一致，否则终端会错位。
	for i, line := range lines {
		if got := len([]rune(line)); got != cols {
			t.Errorf("第 %d 行列数 = %d, want %d", i, got, cols)
		}
	}

	// 必须包含二维码的三种半块字符，说明模块被正确合成。
	if !strings.ContainsAny(view, "█▀▄") {
		t.Error("渲染结果缺少半块字符，可能未正确合成模块")
	}
}

// TestDetectQRModules 校验从真实图片还原出 33×33 模块矩阵。
func TestDetectQRModules(t *testing.T) {
	img := loadQRFixture(t)

	src, err := decodeImage(img)
	if err != nil {
		t.Fatalf("解码图片失败: %v", err)
	}

	grid := detectQRModules(src)
	if grid == nil {
		t.Fatal("未能识别二维码模块结构")
	}

	if grid.size != 33 {
		t.Errorf("模块边长 = %d, want 33（QQ 二维码版本 4）", grid.size)
	}
	if len(grid.modules) != grid.size {
		t.Errorf("行数 = %d, want %d", len(grid.modules), grid.size)
	}

	// 校验三个定位图案（finder pattern）的左上角特征：
	// 7×7 暗框 + 5×5 白框 + 3×3 暗块。
	checkFinder := func(ox, oy int) bool {
		// 外框第 0 行应全暗
		for i := 0; i < 7; i++ {
			if !grid.modules[oy][ox+i] {
				return false
			}
		}
		// 第 1 行第 1 列应亮（白框）
		if grid.modules[oy+1][ox+1] {
			return false
		}
		// 中心 3×3 应全暗
		for dy := 2; dy <= 4; dy++ {
			for dx := 2; dx <= 4; dx++ {
				if !grid.modules[oy+dy][ox+dx] {
					return false
				}
			}
		}
		return true
	}

	if !checkFinder(0, 0) {
		t.Error("左上定位图案特征不符")
	}
	if !checkFinder(grid.size-7, 0) {
		t.Error("右上定位图案特征不符")
	}
	if !checkFinder(0, grid.size-7) {
		t.Error("左下定位图案特征不符")
	}
}

// TestRenderQRToTerminalFallback 校验无法识别模块时仍有输出且不超宽。
func TestRenderQRToTerminalFallback(t *testing.T) {
	// 构造一张非标准尺寸的图（无法整除标准模块数），触发退化路径。
	img := makeGradientImage(100, 100)

	view := renderQRToTerminal(img)
	if view == "" {
		t.Fatal("退化路径应仍有输出")
	}

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	cols := len([]rune(lines[0]))
	if cols > 60 {
		t.Errorf("退化路径宽度 %d 列超出范围", cols)
	}
	t.Logf("退化路径尺寸: %d 列 × %d 行", cols, len(lines))
}
