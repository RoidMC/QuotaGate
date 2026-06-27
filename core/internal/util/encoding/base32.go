package encoding

import (
	"encoding/base32"
	"strings"
)

const (
	// base32Alphabet 是 RFC 4648 §6 定义的标准 Base32 字母表
	// base32Alphabet is the standard Base32 alphabet as defined in RFC 4648 §6.
	base32Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

	// base32HexAlphabet 是 RFC 4648 §7 定义的 Base32 Hex 字母表
	// base32HexAlphabet is the Base32 Hex alphabet as defined in RFC 4648 §7.
	base32HexAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUV"
)

// Base32Std 是 RFC 4648 §6 定义的标准 Base32 编码器（带填充）
// Base32Std is the standard Base32 encoder with padding as defined in RFC 4648 §6.
var Base32Std = base32.StdEncoding

// Base32Hex 是 RFC 4648 §7 定义的 Base32 Hex 编码器（带填充）
// Base32Hex is the Base32 Hex encoder with padding as defined in RFC 4648 §7.
var Base32Hex = base32.HexEncoding

// Base32NoPad 是 RFC 4648 §6 Base32 的无填充变体
// 适用于数据长度已知或通过其他方式确定的场景
// Base32NoPad is the unpadded variant of RFC 4648 §6 Base32.
// Suitable for scenarios where data length is known or determined by other means.
var Base32NoPad = base32.NewEncoding(base32Alphabet).WithPadding(base32.NoPadding)

// Base32HexNoPad 是 RFC 4648 §7 Base32 Hex 的无填充变体
// Base32HexNoPad is the unpadded variant of RFC 4648 §7 Base32 Hex.
var Base32HexNoPad = base32.NewEncoding(base32HexAlphabet).WithPadding(base32.NoPadding)

// Base32Encode 使用 RFC 4648 §6 标准字母表编码数据，不添加填充
// Base32Encode encodes data using the RFC 4648 §6 standard alphabet without padding.
func Base32Encode(data []byte) string {
	return Base32NoPad.EncodeToString(data)
}

// Base32Decode 解码 RFC 4648 §6 标准 Base32 编码的数据
// 接受大写/小写输入，以及带填充或不带填充的格式
// Base32Decode decodes RFC 4648 §6 standard Base32 encoded data.
// Accepts uppercase/lowercase input, with or without padding.
func Base32Decode(s string) ([]byte, error) {
	s = strings.ToUpper(s)
	s = strings.TrimRight(s, "=")
	return Base32NoPad.DecodeString(s)
}

// Base32HexEncode 使用 RFC 4648 §7 Hex 字母表编码数据，不添加填充
// Base32HexEncode encodes data using the RFC 4648 §7 Hex alphabet without padding.
func Base32HexEncode(data []byte) string {
	return Base32HexNoPad.EncodeToString(data)
}

// Base32HexDecode 解码 RFC 4648 §7 Base32 Hex 编码的数据
// 接受大写/小写输入，以及带填充或不带填充的格式
// Base32HexDecode decodes RFC 4648 §7 Base32 Hex encoded data.
// Accepts uppercase/lowercase input, with or without padding.
func Base32HexDecode(s string) ([]byte, error) {
	s = strings.ToUpper(s)
	s = strings.TrimRight(s, "=")
	return Base32HexNoPad.DecodeString(s)
}
