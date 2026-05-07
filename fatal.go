package main

import (
	"errors"
	"fmt"
)

// FatalError marks an exception that retry loops MUST NOT swallow.
// Use for configuration problems (missing binary), workspace corruption,
// anything where retrying the same operation will deterministically fail
// the same way.
type FatalError struct {
	Msg string
}

func (e *FatalError) Error() string {
	return e.Msg
}

func ThrowFatal(format string, args ...any) {
	New(&FatalError{Msg: fmt.Sprintf(format, args...)}).throw()
}

func IsFatal(e *Exception) bool {
	if e == nil {
		return false
	}

	var fe *FatalError

	return errors.As(e.AsError(), &fe)
}

// Rethrow propagates an exception past a Try boundary. Used by retry
// loops to skip caught-but-fatal exceptions back up to main.
func Rethrow(e *Exception) {
	if e != nil {
		panic(e)
	}
}
