package kexrandom

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

var (
	ErrInvalidUUIDLength = errors.New("kexcore/random: uuid invalid byte length")
)

func NewUUID() ([16]byte, error) {
	var uuid [16]byte

	now := time.Now()
	ms := uint64(now.UnixMilli())

	uuid[0] = byte(ms >> 40)
	uuid[1] = byte(ms >> 32)
	uuid[2] = byte(ms >> 24)
	uuid[3] = byte(ms >> 16)
	uuid[4] = byte(ms >> 8)
	uuid[5] = byte(ms)

	uuid[6] = 0x70 | (uuid[6] & 0x0F)

	uuid[8] = 0x80 | (uuid[8] & 0x3F)

	if _, err := rand.Read(uuid[6:8]); err != nil {
		return uuid, err
	}
	uuid[6] = 0x70 | (uuid[6] & 0x0F)

	if _, err := rand.Read(uuid[8:]); err != nil {
		return uuid, err
	}
	uuid[8] = 0x80 | (uuid[8] & 0x3F)

	return uuid, nil
}

func MustUUID() [16]byte {
	uuid, err := NewUUID()
	if err != nil {
		panic(err)
	}
	return uuid
}

func NewUUIDString() (string, error) {
	uuid, err := NewUUID()
	if err != nil {
		return "", err
	}
	return FormatUUID(uuid), nil
}

func MustUUIDString() string {
	return FormatUUID(MustUUID())
}

func FormatUUID(uuid [16]byte) string {
	b := uuid[:]
	return hex.EncodeToString(b[0:4]) + "-" +
		hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" +
		hex.EncodeToString(b[8:10]) + "-" +
		hex.EncodeToString(b[10:])
}

func ParseUUID(s string) ([16]byte, error) {
	var uuid [16]byte

	hexStr := s
	if len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-' {
		hexStr = s[0:8] + s[9:13] + s[14:18] + s[19:23] + s[24:]
	}

	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return uuid, err
	}

	if len(b) != 16 {
		return uuid, ErrInvalidUUIDLength
	}

	copy(uuid[:], b)
	return uuid, nil
}
