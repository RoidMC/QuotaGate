package avatar

import (
	"crypto/md5"
	"fmt"
	"net/url"
	"strings"
)

type GravatarDefault string

const (
	GravatarDefaultIdenticon GravatarDefault = "identicon"
	GravatarDefaultMP        GravatarDefault = "mp"
	GravatarDefaultMonsterID GravatarDefault = "monsterid"
	GravatarDefaultWavatar   GravatarDefault = "wavatar"
	GravatarDefaultRetro     GravatarDefault = "retro"
	GravatarDefaultRoboHash  GravatarDefault = "robohash"
	GravatarDefaultBlank     GravatarDefault = "blank"
	GravatarDefault404       GravatarDefault = "404"
)

type GravatarRating string

const (
	GravatarRatingG  GravatarRating = "g"
	GravatarRatingPG GravatarRating = "pg"
	GravatarRatingR  GravatarRating = "r"
	GravatarRatingX  GravatarRating = "x"
)

type GravatarOption func(*gravatarParams)

type gravatarParams struct {
	size         int
	defaultImage GravatarDefault
	rating       GravatarRating
	forceDefault bool
}

func GravatarWithSize(size int) GravatarOption {
	return func(p *gravatarParams) {
		if size > 0 {
			p.size = size
		}
	}
}

func GravatarWithDefault(d GravatarDefault) GravatarOption {
	return func(p *gravatarParams) {
		p.defaultImage = d
	}
}

func GravatarWithRating(r GravatarRating) GravatarOption {
	return func(p *gravatarParams) {
		p.rating = r
	}
}

func GravatarForceDefault() GravatarOption {
	return func(p *gravatarParams) {
		p.forceDefault = true
	}
}

func GravatarURL(email string, opts ...GravatarOption) string {
	params := &gravatarParams{
		size:         120,
		defaultImage: GravatarDefaultIdenticon,
		rating:       GravatarRatingPG,
	}
	for _, opt := range opts {
		opt(params)
	}

	hash := gravatarHash(email)

	values := url.Values{}
	values.Set("s", fmt.Sprintf("%d", params.size))
	if params.defaultImage != "" {
		values.Set("d", string(params.defaultImage))
	}
	if params.rating != "" {
		values.Set("r", string(params.rating))
	}
	if params.forceDefault {
		values.Set("f", "y")
	}

	return "https://www.gravatar.com/avatar/" + hash + "?" + values.Encode()
}

func gravatarHash(email string) string {
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)
	hash := md5.Sum([]byte(email))
	return fmt.Sprintf("%x", hash)
}
