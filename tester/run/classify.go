package run

import (
	"errors"
	"os"
	"syscall"

	"tester/gnb"
	"tester/metrics"
)

// Classify maps an N2 attempt error to the outcome the stage card counts.
func Classify(err error) metrics.Outcome {
	var rejected *gnb.RejectedError
	switch {
	case err == nil:
		return metrics.Accepted
	case errors.As(err, &rejected):
		return metrics.Rejected
	case errors.Is(err, syscall.EAGAIN), errors.Is(err, syscall.EINPROGRESS),
		errors.Is(err, syscall.ETIMEDOUT), errors.Is(err, os.ErrDeadlineExceeded):
		return metrics.TimedOut
	default:
		return metrics.Failed
	}
}

// CauseOf is the key failures are grouped by: the NGAP cause for a
// rejection, otherwise the innermost error (e.g. "connection refused").
func CauseOf(err error) string {
	var rejected *gnb.RejectedError
	if errors.As(err, &rejected) {
		return rejected.Cause
	}
	for {
		inner := errors.Unwrap(err)
		if inner == nil {
			return err.Error()
		}
		err = inner
	}
}
