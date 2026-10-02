package profile

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// IncrementHex adds step to a hex string and keeps its width, so "000314"
// + 2 is "000316". It fails instead of growing the string on overflow.
func IncrementHex(start string, step int) (string, error) {
	if _, err := hex.DecodeString(start); err != nil || start == "" {
		return "", fmt.Errorf("%q is not an even-length hex string", start)
	}
	v, _ := new(big.Int).SetString(start, 16)
	v.Add(v, big.NewInt(int64(step)))
	out := fmt.Sprintf("%0*x", len(start), v)
	if len(out) > len(start) {
		return "", fmt.Errorf("%q + %d overflows %d hex digits", start, step, len(start))
	}
	return out, nil
}

// IncrementDecimal adds step to a decimal string and keeps its width, so
// "0000000009" + 1 is "0000000010". It fails instead of growing.
func IncrementDecimal(start string, step int) (string, error) {
	if start == "" || strings.Trim(start, "0123456789") != "" {
		return "", fmt.Errorf("%q is not a decimal string", start)
	}
	v, _ := new(big.Int).SetString(start, 10)
	v.Add(v, big.NewInt(int64(step)))
	out := fmt.Sprintf("%0*s", len(start), v.String())
	if len(out) > len(start) {
		return "", fmt.Errorf("%q + %d overflows %d digits", start, step, len(start))
	}
	return out, nil
}

// RenderName replaces every "{i}" in pattern with the 1-based index.
func RenderName(pattern string, index int) string {
	return strings.ReplaceAll(pattern, "{i}", strconv.Itoa(index))
}
