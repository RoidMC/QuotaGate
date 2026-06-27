package avatar_test

import (
	"strings"
	"testing"

	"github.com/roidmc/quotagate/internal/util/avatar"
)

func TestInitialAvatarGenerate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantInit string
	}{
		{"simple name", "John", "J"},
		{"lowercase", "alice", "A"},
		{"with spaces", "  Bob  ", "B"},
		{"chinese", "张三", "张"},
		{"empty", "", "?"},
		{"whitespace only", "   ", "?"},
		{"emoji", "😀Test", "😀"},
		{"numbers first", "123User", "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svg := avatar.GenerateInitial(tt.input)
			if svg == "" {
				t.Fatal("GenerateInitial returned empty string")
			}
			if !strings.HasPrefix(svg, "<svg") {
				t.Errorf("SVG should start with <svg, got: %s", svg[:min(50, len(svg))])
			}
			if !strings.Contains(svg, tt.wantInit) {
				t.Errorf("SVG should contain %q", tt.wantInit)
			}
		})
	}
}

func TestInitialAvatarDeterministic(t *testing.T) {
	name := "TestUser"
	svg1 := avatar.GenerateInitial(name)
	svg2 := avatar.GenerateInitial(name)

	if svg1 != svg2 {
		t.Error("Same name should generate identical initial avatars")
	}
}

func TestInitialAvatarDifferentColors(t *testing.T) {
	svg1 := avatar.GenerateInitial("Alice")
	svg2 := avatar.GenerateInitial("Bob")

	if svg1 == svg2 {
		t.Error("Different names should generate different avatars")
	}
}

func TestInitialAvatarXSSSafety(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"script tag", "<script>alert('xss')</script>"},
		{"onerror", "<img src=x onerror=alert('xss')>"},
		{"svg event", "<svg onload=alert('xss')>"},
		{"ampersand", "Test&User"},
		{"quotes", `Test"User'Name`},
		{"html entities", "&#x22;Test&#x27;"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svg := avatar.GenerateInitial(tt.input)

			if strings.Contains(svg, "<script") {
				t.Error("SVG should not contain unescaped <script")
			}
			if strings.Contains(svg, "onerror") {
				t.Error("SVG should not contain onerror handler")
			}
			if strings.Contains(svg, "onload") {
				t.Error("SVG should not contain onload handler")
			}
		})
	}
}

func TestInitialAvatarWithSize(t *testing.T) {
	tests := []int{50, 100, 200, 500}

	for _, size := range tests {
		t.Run("", func(t *testing.T) {
			av := avatar.NewInitialAvatar(avatar.InitialWithSize(size))
			svg := av.Generate("Test")

			if svg == "" {
				t.Error("Should generate valid SVG for any size")
			}
		})
	}
}

func TestInitialAvatarInvalidSize(t *testing.T) {
	av := avatar.NewInitialAvatar(avatar.InitialWithSize(0))

	svg := av.Generate("Test")
	if svg == "" {
		t.Error("Zero size should still generate valid SVG")
	}
}

func TestInitialAvatarBase64(t *testing.T) {
	result := avatar.GenerateInitialBase64("John")

	if !strings.HasPrefix(result, avatar.SVGBase64Prefix) {
		t.Errorf("Result should start with SVG prefix, got: %s", result[:min(50, len(result))])
	}
}
