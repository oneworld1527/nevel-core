package wallet

import (
	"errors"
	"strings"
)

const bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

func bech32Encode(hrp string, data []byte) (string, error) {
	if hrp == "" {
		return "", errors.New("empty bech32 hrp")
	}
	combined := append([]byte(nil), data...)
	combined = append(combined, bech32CreateChecksum(hrp, data)...)
	var b strings.Builder
	b.WriteString(strings.ToLower(hrp))
	b.WriteByte('1')
	for _, p := range combined {
		if p >= 32 {
			return "", errors.New("invalid bech32 data")
		}
		b.WriteByte(bech32Charset[p])
	}
	return b.String(), nil
}

func bech32Decode(s string) (string, []byte, error) {
	if len(s) < 8 || strings.ToLower(s) != s && strings.ToUpper(s) != s {
		return "", nil, errors.New("invalid bech32 string")
	}
	s = strings.ToLower(s)
	pos := strings.LastIndexByte(s, '1')
	if pos <= 0 || pos+7 > len(s) {
		return "", nil, errors.New("invalid bech32 separator")
	}
	hrp := s[:pos]
	data := make([]byte, len(s)-pos-1)
	for i, c := range []byte(s[pos+1:]) {
		idx := strings.IndexByte(bech32Charset, c)
		if idx < 0 {
			return "", nil, errors.New("invalid bech32 character")
		}
		data[i] = byte(idx)
	}
	if !bech32VerifyChecksum(hrp, data) {
		return "", nil, errors.New("invalid bech32 checksum")
	}
	return hrp, data[:len(data)-6], nil
}

func bech32CreateChecksum(hrp string, data []byte) []byte {
	values := append(bech32HrpExpand(hrp), data...)
	values = append(values, 0, 0, 0, 0, 0, 0)
	polymod := bech32Polymod(values) ^ 1
	ret := make([]byte, 6)
	for i := 0; i < 6; i++ {
		ret[i] = byte((polymod >> uint(5*(5-i))) & 31)
	}
	return ret
}

func bech32VerifyChecksum(hrp string, data []byte) bool {
	return bech32Polymod(append(bech32HrpExpand(hrp), data...)) == 1
}
func bech32HrpExpand(hrp string) []byte {
	ret := make([]byte, 0, len(hrp)*2+1)
	for _, c := range []byte(hrp) {
		ret = append(ret, c>>5)
	}
	ret = append(ret, 0)
	for _, c := range []byte(hrp) {
		ret = append(ret, c&31)
	}
	return ret
}
func bech32Polymod(values []byte) uint32 {
	chk := uint32(1)
	gen := [5]uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	for _, v := range values {
		top := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ uint32(v)
		for i := 0; i < 5; i++ {
			if (top>>uint(i))&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}

func convertBits(data []byte, from, to uint, pad bool) ([]byte, error) {
	acc := uint(0)
	bits := uint(0)
	maxv := uint((1 << to) - 1)
	maxAcc := uint((1 << (from + to - 1)) - 1)
	ret := make([]byte, 0, len(data)*int(from)/int(to))
	for _, value := range data {
		v := uint(value)
		if v>>from != 0 {
			return nil, errors.New("invalid bit group")
		}
		acc = ((acc << from) | v) & maxAcc
		bits += from
		for bits >= to {
			bits -= to
			ret = append(ret, byte((acc>>bits)&maxv))
		}
	}
	if pad {
		if bits > 0 {
			ret = append(ret, byte((acc<<(to-bits))&maxv))
		}
	} else if bits >= from || ((acc<<(to-bits))&maxv) != 0 {
		return nil, errors.New("invalid padding")
	}
	return ret, nil
}
