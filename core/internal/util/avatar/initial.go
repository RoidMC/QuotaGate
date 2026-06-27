package avatar

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	// InitialDefaultSize 是首字母头像默认尺寸
	// InitialDefaultSize is the default size for initial avatar.
	InitialDefaultSize = 120

	// InitialDefaultFontSize 是首字母默认字体大小
	// InitialDefaultFontSize is the default font size for initial avatar.
	InitialDefaultFontSize = 48

	// SVGBase64Prefix 是 SVG Base64 Data URL 前缀
	// SVGBase64Prefix is the SVG Base64 Data URL prefix.
	SVGBase64Prefix = "data:image/svg+xml;base64,"
)

type InitialAvatar struct {
	size     int
	fontSize int
}

type InitialOption func(*InitialAvatar)

func InitialWithSize(size int) InitialOption {
	return func(a *InitialAvatar) {
		if size > 0 {
			a.size = size
		}
	}
}

func InitialWithFontSize(fontSize int) InitialOption {
	return func(a *InitialAvatar) {
		if fontSize > 0 {
			a.fontSize = fontSize
		}
	}
}

func NewInitialAvatar(opts ...InitialOption) *InitialAvatar {
	a := &InitialAvatar{
		size:     InitialDefaultSize,
		fontSize: InitialDefaultFontSize,
	}
	for _, opt := range opts {
		opt(a)
	}
	if a.fontSize > a.size*2/3 {
		a.fontSize = a.size * 2 / 3
	}
	return a
}

func (a *InitialAvatar) Generate(name string) string {
	letter := extractInitial(name)
	bgColor := initialColor(name)

	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d">`+
			`<rect width="%d" height="%d" fill="%s"/>`+
			`<text x="50%%" y="54%%" dominant-baseline="middle" alignment-baseline="middle" text-anchor="middle" `+
			`font-family="system-ui,-apple-system,sans-serif" font-size="%d" font-weight="600" fill="#ffffff">%s</text>`+
			`</svg>`,
		a.size, a.size, a.size, a.size,
		a.size, a.size, bgColor,
		a.fontSize, escapeSVG(letter),
	)
}

func (a *InitialAvatar) GenerateBase64(name string) string {
	svg := a.Generate(name)
	return SVGBase64Prefix + base64.StdEncoding.EncodeToString([]byte(svg))
}

func GenerateInitial(name string) string {
	return NewInitialAvatar().Generate(name)
}

func GenerateInitialBase64(name string) string {
	return NewInitialAvatar().GenerateBase64(name)
}

func extractInitial(name string) string {
	name = strings.TrimSpace(name)

	if name == "" {
		return "?"
	}

	r, _ := utf8.DecodeRuneInString(name)
	initial := strings.ToUpper(string(r))

	return initial
}

func initialColor(name string) string {
	hash := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(name))))
	h := uint64(hash[0])<<8 | uint64(hash[1])

	hue := float64(h % 360)
	s := 0.55 + float64((h>>8)%20)/100.0
	l := 0.45 + float64((h>>16)%15)/100.0

	c := hslToRGB(hue, s, l)
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

func escapeSVG(s string) string {
	var buf bytes.Buffer
	for _, r := range s {
		switch r {
		case '&':
			buf.WriteString("&amp;")
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		case '"':
			buf.WriteString("&quot;")
		case '\'':
			buf.WriteString("&apos;")
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}
