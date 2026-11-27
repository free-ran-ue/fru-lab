package internal

import (
	"testing"
)

func TestValidStreamTarget(t *testing.T) {
	for _, tc := range []struct {
		target, instance string
		want             bool
	}{
		{"free5gc", "", true},
		{"gnb-slice1", "", false},
		{"gnb-3f2a9c1e5b7d", "", true},
		{"gnb", "", false},
		{"gnb-../../x", "", false},
		{"ue", "", false},
		{"ue", "imsi-208930000000001", true},
		{"core-3f2a9c1e5b7d", "", true},
		{"core-x", "", false},
		{"core-../../3f2a9c", "", false},
	} {
		if validStreamTarget(tc.target, tc.instance) != tc.want {
			t.Errorf("%s/%s: want %v", tc.target, tc.instance, tc.want)
		}
	}
}

func TestParseTail(t *testing.T) {
	cases := map[string]int{"all": -1, "0": 0, "300": 300, "5000": 5000, "5001": 200, "-3": 200, "x": 200}
	for q, want := range cases {
		if got := parseTail(q); got != want {
			t.Errorf("parseTail(%q) = %d, want %d", q, got, want)
		}
	}
}
