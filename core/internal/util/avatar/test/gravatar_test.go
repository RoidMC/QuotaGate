package avatar_test

import (
	"strings"
	"testing"

	"github.com/roidmc/quotagate/internal/util/avatar"
)

func TestGravatarURL(t *testing.T) {
	email := "test@example.com"
	url := avatar.GravatarURL(email)

	if !strings.HasPrefix(url, "https://www.gravatar.com/avatar/") {
		t.Errorf("Gravatar URL should start with correct prefix, got: %s", url)
	}
	if !strings.Contains(url, "s=120") {
		t.Error("Gravatar URL should contain default size 120")
	}
	if !strings.Contains(url, "d=identicon") {
		t.Error("Gravatar URL should contain default identicon fallback")
	}
	if !strings.Contains(url, "r=pg") {
		t.Error("Gravatar URL should contain default PG rating")
	}
}

func TestGravatarWithOptions(t *testing.T) {
	email := "user@example.com"
	url := avatar.GravatarURL(email,
		avatar.GravatarWithSize(200),
		avatar.GravatarWithDefault(avatar.GravatarDefaultRetro),
		avatar.GravatarWithRating(avatar.GravatarRatingG),
		avatar.GravatarForceDefault(),
	)

	if !strings.Contains(url, "s=200") {
		t.Error("Gravatar URL should contain custom size 200")
	}
	if !strings.Contains(url, "d=retro") {
		t.Error("Gravatar URL should contain retro default")
	}
	if !strings.Contains(url, "r=g") {
		t.Error("Gravatar URL should contain G rating")
	}
	if !strings.Contains(url, "f=y") {
		t.Error("Gravatar URL should contain force default flag")
	}
}

func TestGravatarHashConsistency(t *testing.T) {
	tests := []struct {
		email1 string
		email2 string
	}{
		{"Test@Example.COM", "test@example.com"},
		{"  test@example.com  ", "test@example.com"},
		{"UPPER@EXAMPLE.COM", "upper@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.email1+"_vs_"+tt.email2, func(t *testing.T) {
			url1 := avatar.GravatarURL(tt.email1)
			url2 := avatar.GravatarURL(tt.email2)

			hash1 := strings.Split(url1, "?")[0]
			hash2 := strings.Split(url2, "?")[0]

			if hash1 != hash2 {
				t.Errorf("Gravatar hash should be case-insensitive:\n%s\n%s", hash1, hash2)
			}
		})
	}
}

func TestGravatarDefaultOptions(t *testing.T) {
	tests := []struct {
		name     string
		default_ avatar.GravatarDefault
		want     string
	}{
		{"identicon", avatar.GravatarDefaultIdenticon, "d=identicon"},
		{"mp", avatar.GravatarDefaultMP, "d=mp"},
		{"monsterid", avatar.GravatarDefaultMonsterID, "d=monsterid"},
		{"wavatar", avatar.GravatarDefaultWavatar, "d=wavatar"},
		{"retro", avatar.GravatarDefaultRetro, "d=retro"},
		{"robohash", avatar.GravatarDefaultRoboHash, "d=robohash"},
		{"blank", avatar.GravatarDefaultBlank, "d=blank"},
		{"404", avatar.GravatarDefault404, "d=404"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := avatar.GravatarURL("test@example.com", avatar.GravatarWithDefault(tt.default_))

			if !strings.Contains(url, tt.want) {
				t.Errorf("URL should contain %s, got: %s", tt.want, url)
			}
		})
	}
}

func TestGravatarRatingOptions(t *testing.T) {
	tests := []struct {
		name   string
		rating avatar.GravatarRating
		want   string
	}{
		{"g", avatar.GravatarRatingG, "r=g"},
		{"pg", avatar.GravatarRatingPG, "r=pg"},
		{"r", avatar.GravatarRatingR, "r=r"},
		{"x", avatar.GravatarRatingX, "r=x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := avatar.GravatarURL("test@example.com", avatar.GravatarWithRating(tt.rating))

			if !strings.Contains(url, tt.want) {
				t.Errorf("URL should contain %s, got: %s", tt.want, url)
			}
		})
	}
}

func TestGravatarEmptyEmail(t *testing.T) {
	url := avatar.GravatarURL("")

	if !strings.HasPrefix(url, "https://www.gravatar.com/avatar/") {
		t.Error("Empty email should still return valid Gravatar URL structure")
	}
}