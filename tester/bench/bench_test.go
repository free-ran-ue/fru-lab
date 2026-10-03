package bench

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSenderCountsDoubleUpToTheCPUs(t *testing.T) {
	require.Equal(t, []int{1}, senderCounts(1))
	require.Equal(t, []int{1, 2, 4, 6}, senderCounts(6))
	require.Equal(t, []int{1, 2, 4, 8}, senderCounts(8))
	require.Equal(t, []int{1, 2, 4, 8, 16, 32, 48}, senderCounts(48))
}

func TestSettingsAreValidated(t *testing.T) {
	r := NewRunner(func() bool { return false })
	for _, bad := range []Settings{{PacketSize: 63, StepSeconds: 3}, {PacketSize: 9001, StepSeconds: 3}, {PacketSize: 1400, StepSeconds: 0}, {PacketSize: 1400, StepSeconds: 31}} {
		_, err := r.Start(bad)
		require.ErrorIs(t, err, ErrInvalidSettings, "%+v", bad)
	}
}

// fakeMeasure reports senders x 100 kpps after a short pause, so the test
// can watch the steps fill in.
func fakeMeasure(gate chan struct{}) func(int, Settings) (Step, error) {
	return func(senders int, s Settings) (Step, error) {
		if gate != nil {
			<-gate
		}
		pps := float64(senders) * 100e3
		return Step{Senders: senders, Pps: pps, Bps: pps * float64(s.PacketSize*8)}, nil
	}
}

func TestARunMeasuresEverySenderCountInOrder(t *testing.T) {
	r := NewRunner(func() bool { return false })
	r.cpus = 4
	r.measure = fakeMeasure(nil)
	res, err := r.Start(Settings{PacketSize: 1400, StepSeconds: 1})
	require.NoError(t, err)
	require.Equal(t, StateRunning, res.State)
	require.Equal(t, []int{1, 2, 4}, res.PlannedSenders)

	require.Eventually(t, func() bool { return r.Result().State == StateDone }, time.Second, 5*time.Millisecond)
	res = r.Result()
	require.Len(t, res.Steps, 3)
	require.Equal(t, []int{1, 2, 4}, []int{res.Steps[0].Senders, res.Steps[1].Senders, res.Steps[2].Senders})
	require.Equal(t, 400e3, res.Steps[2].Pps)
	require.NotNil(t, res.FinishedAt)
	require.False(t, r.Running())
}

func TestOnlyOneBenchAtATimeAndNotDuringARun(t *testing.T) {
	gate := make(chan struct{})
	r := NewRunner(func() bool { return false })
	r.cpus = 2
	r.measure = fakeMeasure(gate)
	_, err := r.Start(Settings{PacketSize: 1400, StepSeconds: 1})
	require.NoError(t, err)
	require.True(t, r.Running())
	_, err = r.Start(Settings{PacketSize: 1400, StepSeconds: 1})
	require.ErrorIs(t, err, ErrBenchRunning)
	close(gate)
	require.Eventually(t, func() bool { return !r.Running() }, time.Second, 5*time.Millisecond)

	busy := NewRunner(func() bool { return true })
	_, err = busy.Start(Settings{PacketSize: 1400, StepSeconds: 1})
	require.ErrorIs(t, err, ErrRunActive)
}

func TestAFailedStepFailsTheBench(t *testing.T) {
	r := NewRunner(func() bool { return false })
	r.cpus = 2
	var once sync.Once
	r.measure = func(senders int, _ Settings) (Step, error) {
		var err error
		once.Do(func() { err = errors.New("bind 127.0.0.11:2152: address already in use") })
		return Step{Senders: senders}, err
	}
	_, err := r.Start(Settings{PacketSize: 1400, StepSeconds: 1})
	require.NoError(t, err)
	require.Eventually(t, func() bool { return r.Result().State == StateFailed }, time.Second, 5*time.Millisecond)
	require.Contains(t, r.Result().Error, "address already in use")
}

// The real measurement: one sender on loopback, briefly, with and
// without UDP GSO.
func TestMeasureSendOnLoopback(t *testing.T) {
	for _, gso := range []bool{false, true} {
		step, err := measureSend(1, Settings{PacketSize: 1400, StepSeconds: 1, Gso: gso}, 100*time.Millisecond, 300*time.Millisecond)
		require.NoError(t, err)
		require.Equal(t, 1, step.Senders)
		require.Greater(t, step.Pps, 1000.0)
		require.InDelta(t, step.Pps*1400*8, step.Bps, 1)
		require.Equal(t, gso, step.Gso, "this kernel supports UDP GSO")
	}
}
