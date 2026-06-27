package avatar_test

import (
	"bytes"
	"image/png"
	"strings"
	"testing"

	"github.com/roidmc/quotagate/internal/util/avatar"
)

func TestGenerate(t *testing.T) {
	tests := []struct {
		name string
		id   int64
	}{
		{"positive id", 12345},
		{"negative id", -12345},
		{"zero id", 0},
		{"large id", 9223372036854775807},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := avatar.Generate(tt.id)
			if err != nil {
				t.Fatalf("Generate(%d) error: %v", tt.id, err)
			}
			if len(data) == 0 {
				t.Fatalf("Generate(%d) returned empty data", tt.id)
			}

			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("Generated data is not valid PNG: %v", err)
			}
			if img.Bounds().Dx() != avatar.DefaultSize {
				t.Errorf("Image width = %d, want %d", img.Bounds().Dx(), avatar.DefaultSize)
			}
			if img.Bounds().Dy() != avatar.DefaultSize {
				t.Errorf("Image height = %d, want %d", img.Bounds().Dy(), avatar.DefaultSize)
			}
		})
	}
}

func TestDeterministic(t *testing.T) {
	id := int64(42)

	data1, err := avatar.Generate(id)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	data2, err := avatar.Generate(id)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	if !bytes.Equal(data1, data2) {
		t.Error("Same id should generate identical avatars")
	}
}

func TestDifferentIds(t *testing.T) {
	data1, err := avatar.Generate(1)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	data2, err := avatar.Generate(2)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	if bytes.Equal(data1, data2) {
		t.Error("Different ids should generate different avatars")
	}
}

func TestGenerateSVG(t *testing.T) {
	id := int64(12345)
	svg := avatar.GenerateSVG(id)

	if svg == "" {
		t.Fatal("GenerateSVG returned empty string")
	}
	if !strings.HasPrefix(svg, "<svg") {
		t.Errorf("SVG should start with <svg tag, got: %s", svg[:min(50, len(svg))])
	}
	if !strings.HasSuffix(svg, "</svg>") {
		t.Errorf("SVG should end with </svg>, got: %s", svg[len(svg)-20:])
	}
}

func TestGenerateSVGDeterministic(t *testing.T) {
	id := int64(42)

	svg1 := avatar.GenerateSVG(id)
	svg2 := avatar.GenerateSVG(id)

	if svg1 != svg2 {
		t.Error("Same id should generate identical SVG avatars")
	}
}

func TestGenerateSVGBase64(t *testing.T) {
	id := int64(12345)
	result := avatar.GenerateSVGBase64(id)

	if !strings.HasPrefix(result, avatar.SVGBase64Prefix) {
		t.Errorf("Result should start with SVG prefix, got: %s", result[:min(50, len(result))])
	}
}

func TestWithOptions(t *testing.T) {
	customSize := 240
	customGrid := 7

	identicon := avatar.NewIdenticon(
		avatar.WithSize(customSize),
		avatar.WithGrid(customGrid),
	)

	data, err := identicon.Generate(42)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Generated data is not valid PNG: %v", err)
	}

	expectedWidth := (customSize / customGrid) * customGrid
	expectedSize := expectedWidth + (customSize/customGrid)/2*2
	if img.Bounds().Dx() != expectedSize {
		t.Errorf("Image width = %d, want %d", img.Bounds().Dx(), expectedSize)
	}
}

func TestEmptyOptions(t *testing.T) {
	identicon := avatar.NewIdenticon()

	data, err := identicon.Generate(42)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	if len(data) == 0 {
		t.Error("Generate returned empty data")
	}
}

func BenchmarkGenerate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = avatar.Generate(int64(i))
	}
}

func BenchmarkGenerateSVG(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = avatar.GenerateSVG(int64(i))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
