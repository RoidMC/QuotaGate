package kexavatar_test

import (
	"strings"
	"testing"

	"github.com/roidmc/kex-utils/pkg/kexavatar"
)

func FuzzIdenticonGenerate(f *testing.F) {
	ids := []int64{0, 1, -1, 42, 12345, -999999, 9223372036854775807, -9223372036854775808}
	for _, id := range ids {
		f.Add(id)
	}

	f.Fuzz(func(t *testing.T, id int64) {
		data, err := kexavatar.Generate(id)
		if err != nil {
			t.Fatalf("Generate(%d) failed: %v", id, err)
		}
		if len(data) == 0 {
			t.Fatalf("Generate(%d) returned empty data", id)
		}
	})
}

func FuzzIdenticonSVG(f *testing.F) {
	ids := []int64{0, 1, -1, 42, 12345, -999999, 9223372036854775807, -9223372036854775808}
	for _, id := range ids {
		f.Add(id)
	}

	f.Fuzz(func(t *testing.T, id int64) {
		svg := kexavatar.GenerateSVG(id)
		if svg == "" {
			t.Fatalf("GenerateSVG(%d) returned empty string", id)
		}
		if !strings.HasPrefix(svg, "<svg") {
			t.Fatalf("GenerateSVG(%d) invalid SVG start: %s", id, svg[:min(50, len(svg))])
		}
		if !strings.HasSuffix(svg, "</svg>") {
			t.Fatalf("GenerateSVG(%d) invalid SVG end: %s", id, svg[len(svg)-20:])
		}
	})
}

func FuzzInitialAvatar(f *testing.F) {
	f.Add("John")
	f.Add("alice")
	f.Add("张三")
	f.Add("")
	f.Add("   ")
	f.Add("<script>alert('xss')</script>")
	f.Add("A")
	f.Add("😀")

	f.Fuzz(func(t *testing.T, name string) {
		svg := kexavatar.GenerateInitial(name)

		if svg == "" {
			t.Fatalf("GenerateInitial(%q) returned empty string", name)
		}
		if !strings.HasPrefix(svg, "<svg") {
			t.Fatalf("GenerateInitial(%q) invalid SVG start", name)
		}
		if !strings.HasSuffix(svg, "</svg>") {
			t.Fatalf("GenerateInitial(%q) invalid SVG end", name)
		}
		if strings.Contains(svg, "<script") {
			t.Fatalf("GenerateInitial(%q) contains unescaped script tag", name)
		}
		if strings.Contains(svg, "onerror") {
			t.Fatalf("GenerateInitial(%q) contains onerror handler", name)
		}
		if strings.Contains(svg, "onload") {
			t.Fatalf("GenerateInitial(%q) contains onload handler", name)
		}
	})
}

func FuzzInitialAvatarXSS(f *testing.F) {
	attackVectors := []string{
		"<script>alert('xss')</script>",
		"<img src=x onerror=alert('xss')>",
		"<svg onload=alert('xss')>",
		"<body onload=alert('xss')>",
		"<iframe src=javascript:alert('xss')>",
		"<input onfocus=alert('xss') autofocus>",
		"<details open ontoggle=alert('xss')>",
		"\x22\x3e\x3cscript\x3ealert(String.fromCharCode(88,83,83))\x3c/script\x3e",
		"' onfocus=alert('xss') autofocus='",
		"\x00\x01\x02\x03",
		"\n\r\t<script>",
	}
	for _, v := range attackVectors {
		f.Add(v)
	}

	f.Fuzz(func(t *testing.T, input string) {
		svg := kexavatar.GenerateInitial(input)

		if strings.Contains(svg, "<script") {
			t.Fatalf("XSS: unescaped <script> in output for input: %q", input)
		}
		if strings.Contains(svg, "onerror") || strings.Contains(svg, "onload") {
			t.Fatalf("XSS: unescaped event handler in output for input: %q", input)
		}
		if strings.Contains(svg, "<iframe") || strings.Contains(svg, "<body") {
			t.Fatalf("XSS: unescaped HTML tag in output for input: %q", input)
		}
	})
}

func FuzzGravatarURL(f *testing.F) {
	f.Add("test@example.com")
	f.Add("")
	f.Add("UPPER@EXAMPLE.COM")
	f.Add("   spaced@example.com   ")
	f.Add("user+tag@example.com")
	f.Add("user@sub.domain.example.com")

	f.Fuzz(func(t *testing.T, email string) {
		url := kexavatar.GravatarURL(email)

		if url == "" {
			t.Fatalf("GravatarURL(%q) returned empty string", email)
		}
		if !strings.HasPrefix(url, "https://www.gravatar.com/avatar/") {
			t.Fatalf("GravatarURL(%q) invalid prefix: %s", email, url)
		}
		if strings.Contains(url, "\x3c") || strings.Contains(url, "\x3e") {
			t.Fatalf("GravatarURL(%q) contains angle brackets: %s", email, url)
		}
		if strings.Contains(url, " ") {
			t.Fatalf("GravatarURL(%q) contains spaces: %s", email, url)
		}
	})
}

func FuzzGravatarHashConsistency(f *testing.F) {
	emails := []string{
		"Test@Example.COM",
		" test@example.com ",
		"TEST@EXAMPLE.COM",
	}
	for _, e := range emails {
		f.Add(e)
	}

	f.Fuzz(func(t *testing.T, email string) {
		url1 := kexavatar.GravatarURL(email)
		url2 := kexavatar.GravatarURL(strings.ToLower(strings.TrimSpace(email)))

		hash1 := strings.Split(url1, "?")[0]
		hash2 := strings.Split(url2, "?")[0]

		if hash1 != hash2 {
			t.Fatalf("Hash mismatch for %q vs normalized: %s vs %s", email, hash1, hash2)
		}
	})
}

func FuzzDeterministicIdenticon(f *testing.F) {
	f.Add(int64(42))
	f.Add(int64(0))
	f.Add(int64(-1))

	f.Fuzz(func(t *testing.T, id int64) {
		png1, _ := kexavatar.Generate(id)
		png2, _ := kexavatar.Generate(id)
		if string(png1) != string(png2) {
			t.Fatalf("Identicon PNG not deterministic for id=%d", id)
		}

		svg1 := kexavatar.GenerateSVG(id)
		svg2 := kexavatar.GenerateSVG(id)
		if svg1 != svg2 {
			t.Fatalf("Identicon SVG not deterministic for id=%d", id)
		}
	})
}

func FuzzDeterministicInitial(f *testing.F) {
	f.Add("TestUser")
	f.Add("alice")
	f.Add("张三")

	f.Fuzz(func(t *testing.T, name string) {
		svg1 := kexavatar.GenerateInitial(name)
		svg2 := kexavatar.GenerateInitial(name)
		if svg1 != svg2 {
			t.Fatalf("Initial avatar not deterministic for name=%q", name)
		}
	})
}
