package app

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/mdp/qrterminal/v3"
	"github.com/skip2/go-qrcode"
)

func GenQRCode(filename, content string) (path string, buffer bytes.Buffer, err error) {
	localDir := RuntimeDir()
	path = filepath.Join(localDir, filename)
	if err = qrcode.WriteFile(content, qrcode.Medium, 256, path); err != nil {
		return "", bytes.Buffer{}, err
	}
	config := qrterminal.Config{
		Level:      qrterminal.L,
		Writer:     &buffer,
		HalfBlocks: true,
		QuietZone:  1,
	}
	qrterminal.GenerateWithConfig(content, config)
	return path, buffer, nil
}

// decodeImage 从字节解码图片，供内部与测试共用。
func decodeImage(data []byte) (image.Image, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return src, nil
}

// GenQRCodeFromImage 将服务端返回的二维码图片落盘，并渲染为终端字符画。
//
// 与 GenQRCode 的区别：内容不是待编码的字符串，而是已编码好的图片字节
// （QQ / 微信的扫码登录接口直接返回图片）。渲染时按亮度二值化后用半块字符
// 拼接，保证在终端里可扫。
func GenQRCodeFromImage(filename string, img []byte) (path string, buffer bytes.Buffer, err error) {
	localDir := RuntimeDir()
	path = filepath.Join(localDir, filename)
	if err = os.WriteFile(path, img, 0o600); err != nil {
		return "", bytes.Buffer{}, err
	}

	src, err := decodeImage(img)
	if err != nil {
		return "", bytes.Buffer{}, fmt.Errorf("解码二维码图片失败: %w", err)
	}

	buffer.WriteString(renderQRToTerminal(src))
	return path, buffer, nil
}

// qrModuleGrid 是从位图还原出的二维码模块矩阵。
type qrModuleGrid struct {
	// modules[y][x] 为 true 表示该模块是暗块。
	modules [][]bool
	// size 是模块边长（21/25/29/33/...）。
	size int
}

// renderQRToTerminal 把二维码图片渲染为适配终端的半块字符画。
//
// 直接按像素渲染会严重超宽：QQ 的二维码是 111x111 像素（33 模块 × 3 像素/模块
// + 6 像素静区），逐像素输出会得到 111 列 × 56 行，远超终端宽度。
//
// 正确做法是先还原出模块矩阵（33×33），再按每两个模块行合成一个字符行，
// 最终只需 33 列 × 17 行，任何终端都能完整显示。
func renderQRToTerminal(src image.Image) string {
	grid := detectQRModules(src)
	if grid == nil {
		// 无法识别模块结构时退化为按比例缩放的像素渲染，保证仍有输出。
		return renderHalfBlockQRScaled(src, 41)
	}

	var sb strings.Builder
	// 每两个模块行合成一行字符：上暗下暗 → █，上暗下亮 → ▀，
	// 上亮下暗 → ▄，上亮下亮 → 空格。
	for y := 0; y < grid.size; y += 2 {
		for x := 0; x < grid.size; x++ {
			top := grid.modules[y][x]
			bottom := y+1 < grid.size && grid.modules[y+1][x]
			switch {
			case top && bottom:
				sb.WriteString("█")
			case top:
				sb.WriteString("▀")
			case bottom:
				sb.WriteString("▄")
			default:
				sb.WriteString(" ")
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// detectQRModules 从位图中还原二维码模块矩阵。
//
// 步骤：二值化 → 求暗像素包围盒（即二维码本体，排除静区）→ 按标准 QR
// 模块数（21 + 4n）推断模块边长 → 采样每个模块中心点。
// 识别失败返回 nil，由调用方退化处理。
func detectQRModules(src image.Image) *qrModuleGrid {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w < 21 || h < 21 {
		return nil
	}

	isDark := func(x, y int) bool {
		r, g, b, _ := src.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
		lum := (299*uint64(r) + 587*uint64(g) + 114*uint64(b)) / 1000
		return lum < 0x8000
	}

	// 求暗像素包围盒，排除四周静区。
	minX, minY, maxX, maxY := w, h, -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if isDark(x, y) {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < 0 || maxY < 0 {
		return nil
	}

	bodyW := maxX - minX + 1
	bodyH := maxY - minY + 1
	if bodyW != bodyH {
		// 非正方形：可能是带装饰的二维码，放弃模块还原。
		return nil
	}

	// 标准 QR 模块数为 21 + 4n（21/25/29/33/...），
	// 且模块边长应为整数（bodyW 能被模块数整除）。
	size := 0
	for n := 0; n <= 39; n++ {
		candidate := 21 + 4*n
		if candidate > bodyW {
			break
		}
		if bodyW%candidate == 0 {
			size = candidate
		}
	}
	if size == 0 {
		return nil
	}

	modulePx := bodyW / size
	grid := &qrModuleGrid{size: size, modules: make([][]bool, size)}
	for y := 0; y < size; y++ {
		row := make([]bool, size)
		for x := 0; x < size; x++ {
			// 取模块中心点采样，避免边界抗锯齿干扰。
			sx := minX + x*modulePx + modulePx/2
			sy := minY + y*modulePx + modulePx/2
			row[x] = isDark(sx, sy)
		}
		grid.modules[y] = row
	}
	return grid
}

// renderHalfBlockQRScaled 在无法识别模块结构时，把图片按目标宽度缩放后渲染。
//
// targetCols 是期望的字符列数；纵向按 2 像素 1 字符合成，因此行数约为
// targetCols/2，保证不超出常见终端高度。
func renderHalfBlockQRScaled(src image.Image, targetCols int) string {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 || targetCols <= 0 {
		return ""
	}

	isDark := func(x, y int) bool {
		r, g, b, _ := src.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
		lum := (299*uint64(r) + 587*uint64(g) + 114*uint64(b)) / 1000
		return lum < 0x8000
	}

	// 最近邻采样：把源图映射到 targetCols 列、targetCols 像素行（= targetCols/2 字符行）。
	rows := targetCols
	var sb strings.Builder
	for cy := 0; cy < rows; cy += 2 {
		for cx := 0; cx < targetCols; cx++ {
			sample := func(px, py int) bool {
				sx := px * w / targetCols
				sy := py * h / rows
				return isDark(sx, sy)
			}
			top := sample(cx, cy)
			bottom := sample(cx, cy+1)
			switch {
			case top && bottom:
				sb.WriteString("█")
			case top:
				sb.WriteString("▀")
			case bottom:
				sb.WriteString("▄")
			default:
				sb.WriteString(" ")
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
