package music

import (
	"errors"
	"fmt"
	"testing"
)

// A refusal built with Fail is still ErrRefused, through wrapping, and gives
// back the source's own words: the player keeps the track for the first and
// shows the second, as it did for NetEase's -460.
func TestFailKeepsItsKindAndItsReason(t *testing.T) {
	err := fmt.Errorf("loading: %w", Fail(ErrRefused, "检测到您的网络环境存在风险"))
	if !errors.Is(err, ErrRefused) || errors.Is(err, ErrUnavailable) {
		t.Fatalf("%v is not a refusal alone", err)
	}
	if got := Reason(err); got != "检测到您的网络环境存在风险" {
		t.Errorf("reason %q", got)
	}
	if Fail(ErrSetup, "") != ErrSetup || Reason(ErrSetup) != "" {
		t.Error("a failure with no reason is its kind, and has none to give")
	}
}
