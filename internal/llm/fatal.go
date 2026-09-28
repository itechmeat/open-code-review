// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import "errors"

// ErrFatalForRun matches, through errors.Is, a provider error after which no
// further request of the same run can succeed, such as an exhausted
// subscription window. Callers stop dispatching new work on it instead of
// failing every remaining item one by one.
var ErrFatalForRun = errors.New("llm: provider cannot serve further requests in this run")

// runFatalError is a sentinel that also matches ErrFatalForRun, so errors.Is
// checks against the specific sentinel keep working unchanged.
type runFatalError struct{ msg string }

func (e *runFatalError) Error() string { return e.msg }

func (e *runFatalError) Is(target error) bool { return target == ErrFatalForRun }

// fatalDetail pairs a run-fatal sentinel with the provider's own message.
type fatalDetail struct {
	sentinel error
	detail   string
}

func (e *fatalDetail) Error() string { return e.sentinel.Error() + ": " + e.detail }

func (e *fatalDetail) Unwrap() error { return e.sentinel }

// FatalCause returns the part of err that made it fatal for the run, without
// the context added on its way up, or err itself when it is not run-fatal.
func FatalCause(err error) error {
	var d *fatalDetail
	if errors.As(err, &d) {
		return d
	}
	var f *runFatalError
	if errors.As(err, &f) {
		return f
	}
	return err
}
