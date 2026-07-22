package kexavatar

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strconv"
)

const (
	// DefaultGridWidth 是默认网格单元宽度（像素）
	// DefaultGridWidth is the default grid cell width in pixels.
	DefaultGridWidth = 24

	// DefaultGrid 是默认网格数（奇数便于对称）
	// DefaultGrid is the default grid count (odd for symmetry).
	DefaultGrid = 5

	// DefaultPadding 是默认边距（像素）
	// DefaultPadding is the default padding in pixels.
	DefaultPadding = DefaultGridWidth / 2

	// DefaultSize 是默认头像总尺寸（像素）
	// DefaultSize is the default total avatar size in pixels.
	DefaultSize = DefaultGridWidth*DefaultGrid + DefaultPadding*2

	// Base64Prefix 是 Base64 图片数据的前缀
	// Base64Prefix is the prefix for Base64 image data.
	Base64Prefix = "data:image/png;base64,"
)

// Identicon 表示一个确定性头像生成器
// Identicon represents a deterministic avatar generator.
type Identicon struct {
	size    int
	grid    int
	width   int
	padding int
}

// Option 是 Identicon 的配置选项函数
// Option is a configuration option function for Identicon.
type Option func(*Identicon)

// WithSize 设置头像尺寸
// WithSize sets the avatar size.
func WithSize(size int) Option {
	return func(i *Identicon) {
		if size > 0 {
			i.size = size
		}
	}
}

// WithGrid 设置网格数（推荐奇数，便于对称）
// WithGrid sets the grid count (odd numbers recommended for symmetry).
func WithGrid(grid int) Option {
	return func(i *Identicon) {
		if grid > 0 {
			i.grid = grid
		}
	}
}

// NewIdenticon 创建一个新的 Identicon 生成器
// NewIdenticon creates a new Identicon generator.
func NewIdenticon(opts ...Option) *Identicon {
	i := &Identicon{
		grid: DefaultGrid,
	}
	for _, opt := range opts {
		opt(i)
	}
	if i.size == 0 {
		i.size = DefaultSize
		i.width = DefaultGridWidth
		i.padding = DefaultPadding
	} else {
		i.width = i.size / i.grid
		i.padding = i.width / 2
		i.size = i.width*i.grid + i.padding*2
	}
	return i
}

// Generate 根据 id 生成 PNG 格式的头像字节数据
// Generate generates PNG format avatar bytes based on the id.
func (i *Identicon) Generate(id int64) ([]byte, error) {
	img := i.generateImage(id)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GenerateBase64 根据 id 生成 Base64 编码的头像字符串（不含前缀）
// GenerateBase64 generates Base64 encoded avatar string based on the id (without prefix).
func (i *Identicon) GenerateBase64(id int64) (string, error) {
	data, err := i.Generate(id)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// GenerateBase64WithPrefix 根据 id 生成带前缀的 Base64 编码头像字符串
// GenerateBase64WithPrefix generates Base64 encoded avatar string with prefix.
func (i *Identicon) GenerateBase64WithPrefix(id int64) (string, error) {
	encoded, err := i.GenerateBase64(id)
	if err != nil {
		return "", err
	}
	return Base64Prefix + encoded, nil
}

// GenerateSVG 根据 id 生成 SVG 格式的头像字符串
// GenerateSVG generates SVG format avatar string based on the id.
func (i *Identicon) GenerateSVG(id int64) string {
	fgColor := i.deterministicColor(id)
	hexColor := formatHexColor(fgColor)

	pattern := i.generatePattern(id)

	var buf bytes.Buffer
	buf.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 `)
	buf.WriteString(strconv.Itoa(i.grid))
	buf.WriteString(` `)
	buf.WriteString(strconv.Itoa(i.grid))
	buf.WriteString(`" width="` + strconv.Itoa(i.size) + `" height="` + strconv.Itoa(i.size) + `">`)
	buf.WriteString(`<rect width="` + strconv.Itoa(i.grid) + `" height="` + strconv.Itoa(i.grid) + `" fill="#ffffff"/>`)

	for y := 0; y < i.grid; y++ {
		for x := 0; x < i.grid; x++ {
			if pattern[y][x] {
				buf.WriteString(`<rect x="`)
				buf.WriteString(strconv.Itoa(x))
				buf.WriteString(`" y="`)
				buf.WriteString(strconv.Itoa(y))
				buf.WriteString(`" width="1" height="1" fill="`)
				buf.WriteString(hexColor)
				buf.WriteString(`"/>`)
			}
		}
	}

	buf.WriteString(`</svg>`)
	return buf.String()
}

// GenerateSVGBase64 根据 id 生成 Base64 编码的 SVG 头像字符串（含前缀）
// GenerateSVGBase64 generates Base64 encoded SVG avatar string with prefix.
func (i *Identicon) GenerateSVGBase64(id int64) string {
	svg := i.GenerateSVG(id)
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}

// formatHexColor 将 color.RGBA 转换为 #RRGGBB 格式
// formatHexColor converts color.RGBA to #RRGGBB format.
func formatHexColor(c color.RGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// generateImage 根据 id 生成图像
// generateImage generates an image based on the id.
func (i *Identicon) generateImage(id int64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, i.size, i.size))

	bgColor := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	fillRect(img, image.Rect(0, 0, i.size, i.size), bgColor)

	fgColor := i.deterministicColor(id)

	pattern := i.generatePattern(id)
	for y := 0; y < i.grid; y++ {
		for x := 0; x < i.grid; x++ {
			if pattern[y][x] {
				rect := image.Rect(
					i.padding+x*i.width,
					i.padding+y*i.width,
					i.padding+(x+1)*i.width,
					i.padding+(y+1)*i.width,
				)
				fillRect(img, rect, fgColor)
			}
		}
	}

	return img
}

// generatePattern 生成对称图案
// generatePattern generates a symmetric pattern.
func (i *Identicon) generatePattern(id int64) [][]bool {
	pattern := make([][]bool, i.grid)
	for y := range pattern {
		pattern[y] = make([]bool, i.grid)
	}

	hash := i.hashID(id)
	mid := i.grid / 2

	for y := 0; y < i.grid; y++ {
		for dx := 0; dx <= mid; dx++ {
			idx := y*(mid+1) + dx
			bit := (hash >> idx) & 1
			pattern[y][dx] = bit == 1
			mirrorX := i.grid - 1 - dx
			if mirrorX != dx {
				pattern[y][mirrorX] = pattern[y][dx]
			}
		}
	}

	return pattern
}

// hashID 将 id 转换为一个确定性的哈希值
// hashID converts id to a deterministic hash value.
func (i *Identicon) hashID(id int64) uint64 {
	hash := uint64(id)
	hash ^= hash >> 33
	hash *= 0xff51afd7ed558ccd
	hash ^= hash >> 33
	hash *= 0xc4ceb9fe1a85ec53
	hash ^= hash >> 33
	return hash
}

// deterministicColor 根据 id 生成确定性的颜色
// deterministicColor generates a deterministic color based on id.
func (i *Identicon) deterministicColor(id int64) color.RGBA {
	hash := i.hashID(id)

	h := float64(hash % 360)
	s := 0.5 + float64((hash>>8)%30)/100.0
	l := 0.4 + float64((hash>>16)%20)/100.0

	return hslToRGB(h, s, l)
}

// fillRect 填充矩形区域
// fillRect fills a rectangular area.
func fillRect(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

// hslToRGB 将 HSL 颜色转换为 RGB
// hslToRGB converts HSL color to RGB.
func hslToRGB(h, s, l float64) color.RGBA {
	var r, g, b float64

	if s == 0 {
		r, g, b = l, l, l
	} else {
		var q float64
		if l < 0.5 {
			q = l * (1 + s)
		} else {
			q = l + s - l*s
		}
		p := 2*l - q
		r = hueToRGB(p, q, h+120)
		g = hueToRGB(p, q, h)
		b = hueToRGB(p, q, h-120)
	}

	return color.RGBA{
		R: uint8(math.Round(r * 255)),
		G: uint8(math.Round(g * 255)),
		B: uint8(math.Round(b * 255)),
		A: 255,
	}
}

// hueToRGB 辅助函数，将色相转换为 RGB 分量
// hueToRGB is a helper function that converts hue to RGB component.
func hueToRGB(p, q, h float64) float64 {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}

	switch {
	case h < 60:
		return p + (q-p)*h/60
	case h < 180:
		return q
	case h < 240:
		return p + (q-p)*(240-h)/60
	default:
		return p
	}
}

// Generate 是包级别的便捷函数，使用默认配置生成头像
// Generate is a package-level convenience function that generates avatar with default config.
func Generate(id int64) ([]byte, error) {
	return NewIdenticon().Generate(id)
}

// GenerateBase64 是包级别的便捷函数，使用默认配置生成 Base64 头像
// GenerateBase64 is a package-level convenience function that generates Base64 avatar with default config.
func GenerateBase64(id int64) (string, error) {
	return NewIdenticon().GenerateBase64(id)
}

// GenerateBase64WithPrefix 是包级别的便捷函数，使用默认配置生成带前缀的 Base64 头像
// GenerateBase64WithPrefix is a package-level convenience function that generates Base64 avatar with prefix.
func GenerateBase64WithPrefix(id int64) (string, error) {
	return NewIdenticon().GenerateBase64WithPrefix(id)
}

// MustGenerate 生成头像，失败时 panic
// MustGenerate generates avatar, panicking on error.
func MustGenerate(id int64) []byte {
	data, err := Generate(id)
	if err != nil {
		panic(err)
	}
	return data
}

// MustGenerateBase64 生成 Base64 头像，失败时 panic
// MustGenerateBase64 generates Base64 avatar, panicking on error.
func MustGenerateBase64(id int64) string {
	encoded, err := GenerateBase64(id)
	if err != nil {
		panic(err)
	}
	return encoded
}

// MustGenerateBase64WithPrefix 生成带前缀的 Base64 头像，失败时 panic
// MustGenerateBase64WithPrefix generates Base64 avatar with prefix, panicking on error.
func MustGenerateBase64WithPrefix(id int64) string {
	result, err := GenerateBase64WithPrefix(id)
	if err != nil {
		panic(err)
	}
	return result
}

// GenerateSVG 是包级别的便捷函数，使用默认配置生成 SVG 头像
// GenerateSVG is a package-level convenience function that generates SVG avatar with default config.
func GenerateSVG(id int64) string {
	return NewIdenticon().GenerateSVG(id)
}

// GenerateSVGBase64 是包级别的便捷函数，使用默认配置生成 Base64 编码的 SVG 头像
// GenerateSVGBase64 is a package-level convenience function that generates Base64 encoded SVG avatar.
func GenerateSVGBase64(id int64) string {
	return NewIdenticon().GenerateSVGBase64(id)
}
