package profile

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIncrementHex(t *testing.T) {
	cases := []struct {
		start string
		step  int
		want  string
	}{
		{"000314", 0, "000314"},
		{"000314", 1, "000315"},
		{"0003ff", 1, "000400"},
		{"00000001", 9, "0000000a"},
	}
	for _, c := range cases {
		got, err := IncrementHex(c.start, c.step)
		require.NoError(t, err)
		require.Equal(t, c.want, got)
	}
}

func TestIncrementHexRejectsOverflowAndBadInput(t *testing.T) {
	_, err := IncrementHex("ffffff", 1)
	require.ErrorContains(t, err, "overflows")
	_, err = IncrementHex("00031", 0)
	require.ErrorContains(t, err, "hex")
	_, err = IncrementHex("zz", 0)
	require.ErrorContains(t, err, "hex")
	_, err = IncrementHex("", 0)
	require.ErrorContains(t, err, "hex")
}

func TestRenderName(t *testing.T) {
	require.Equal(t, "gNB-3", RenderName("gNB-{i}", 3))
	require.Equal(t, "g3-3", RenderName("g{i}-{i}", 3))
}

func TestIncrementDecimal(t *testing.T) {
	got, err := IncrementDecimal("0000000009", 1)
	require.NoError(t, err)
	require.Equal(t, "0000000010", got)
	got, err = IncrementDecimal("0000000001", 999)
	require.NoError(t, err)
	require.Equal(t, "0000001000", got)

	_, err = IncrementDecimal("9999999999", 1)
	require.ErrorContains(t, err, "overflows")
	_, err = IncrementDecimal("12a", 0)
	require.ErrorContains(t, err, "decimal")
	_, err = IncrementDecimal("", 0)
	require.ErrorContains(t, err, "decimal")
}
