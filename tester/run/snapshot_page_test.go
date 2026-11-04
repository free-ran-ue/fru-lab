package run

import (
	"testing"

	"github.com/stretchr/testify/require"

	"tester/profile"
)

func TestGnbPagePick(t *testing.T) {
	var gnbs []GnbStatus
	for i, st := range []GnbState{GnbUp, GnbFailed, GnbUp, GnbLost, GnbUp, GnbFailed} {
		gnbs = append(gnbs, GnbStatus{GnbSpec: profile.GnbSpec{Index: i + 1}, State: st})
	}
	idx := func(p []GnbStatus) (out []int) {
		for _, g := range p {
			out = append(out, g.Index)
		}
		return out
	}
	page, total, counts := AllGnbs.pick(gnbs)
	require.Equal(t, []int{1, 2, 3, 4, 5, 6}, idx(page))
	require.Equal(t, 6, total)
	require.Equal(t, map[GnbState]int{GnbUp: 3, GnbFailed: 2, GnbLost: 1}, counts)

	page, total, _ = GnbPage{Offset: 2, Limit: 2}.pick(gnbs)
	require.Equal(t, []int{3, 4}, idx(page))
	require.Equal(t, 6, total)

	page, total, counts = GnbPage{Offset: 1, Limit: 5, States: []GnbState{GnbFailed, GnbLost}}.pick(gnbs)
	require.Equal(t, []int{4, 6}, idx(page), "offset counts within the matching gNBs")
	require.Equal(t, 3, total)
	require.Equal(t, 3, counts[GnbUp], "counts cover every gNB")

	page, total, _ = GnbPage{Limit: 0}.pick(gnbs)
	require.Empty(t, page)
	require.NotNil(t, page, "an empty page encodes as []")
	require.Equal(t, 6, total)
}
