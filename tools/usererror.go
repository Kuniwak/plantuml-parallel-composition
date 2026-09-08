package tools

import "errors"

// UserFacing is implemented by an error whose own message is the one to show.
// Unwrapping stops there: it has already been written for the reader, and going
// past it would drop what it adds. csdf.PromotionHintError is one - it wraps a
// parse error so that a caller can still reach it, and says what the author is
// missing on top.
type UserFacing interface{ UserFacing() }

// UserFacingError formats err for end-user display. With debug it returns the
// full wrapped chain (err.Error()); otherwise it unwraps to the deepest error
// that is either the last one or a UserFacing one, and returns its message,
// hiding internal package-qualified context. A nil error yields the empty
// string.
func UserFacingError(err error, debug bool) string {
	if err == nil {
		return ""
	}
	if debug {
		return err.Error()
	}
	for {
		if _, ok := err.(UserFacing); ok {
			return err.Error()
		}
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err.Error()
		}
		err = unwrapped
	}
}
