package tools

import (
	"errors"
	"fmt"
	"testing"
)

func TestUserFacingError(t *testing.T) {
	wrapped := fmt.Errorf("csdf.SolveJSON: invalid JSON array: %w", errors.New("top-level value must be an array"))
	deep := fmt.Errorf("a: %w", fmt.Errorf("b: %w", errors.New("c")))

	tests := []struct {
		name  string
		err   error
		debug bool
		want  string
	}{
		{name: "nil", err: nil, debug: false, want: ""},
		{name: "nil debug", err: nil, debug: true, want: ""},
		{name: "single wrap, no debug, unwraps to leaf", err: wrapped, debug: false, want: "top-level value must be an array"},
		{name: "single wrap, debug, full chain", err: wrapped, debug: true, want: "csdf.SolveJSON: invalid JSON array: top-level value must be an array"},
		{name: "multi wrap, no debug, deepest", err: deep, debug: false, want: "c"},
		{name: "multi wrap, debug, full chain", err: deep, debug: true, want: "a: b: c"},
		{name: "unwrapped leaf, no debug", err: errors.New("plain"), debug: false, want: "plain"},
		{name: "unwrapped leaf, debug", err: errors.New("plain"), debug: true, want: "plain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UserFacingError(tt.err, tt.debug); got != tt.want {
				t.Errorf("UserFacingError(%v, %v) = %q, want %q", tt.err, tt.debug, got, tt.want)
			}
		})
	}
}

type userFacingError struct{ err error }

func (e *userFacingError) Error() string { return e.err.Error() + ": and here is what to do" }
func (e *userFacingError) Unwrap() error { return e.err }
func (e *userFacingError) UserFacing()   {}

// An error that says it is written for the reader is where unwrapping stops.
// Otherwise what it adds would be dropped on the way to the terminal.
func TestUserFacingErrorStopsAtAUserFacingError(t *testing.T) {
	err := fmt.Errorf("outer: %w", &userFacingError{err: errors.New("inner")})

	if got, want := UserFacingError(err, false), "inner: and here is what to do"; got != want {
		t.Errorf("UserFacingError() = %q, want %q", got, want)
	}
}
