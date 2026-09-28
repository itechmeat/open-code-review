// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

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

// RunStop records the first provider error that makes every further request
// of a run fail, so dispatch loops can stop on it. It is safe for concurrent
// use; the zero value is ready.
type RunStop struct {
	mu  sync.Mutex
	err error
}

// Record stores err when it is fatal for the run. It reports whether err is
// fatal and whether it is the first fatal error, the one to announce.
func (s *RunStop) Record(err error) (fatal, first bool) {
	if err == nil || !errors.Is(err, ErrFatalForRun) {
		return false, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return true, false
	}
	s.err = err
	return true, true
}

// Err returns the recorded fatal error, or nil.
func (s *RunStop) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// RunStopMessage describes why dispatch stopped early, or returns "" when
// err is nil. provider names the provider; empty means an unnamed one.
func RunStopMessage(provider string, err error) string {
	if err == nil {
		return ""
	}
	if provider == "" {
		provider = "the LLM provider"
	}
	// Provider sentinels already start with the provider name.
	cause := strings.TrimPrefix(FatalCause(err).Error(), provider+": ")
	return fmt.Sprintf("%s cannot serve further requests, so the remaining files were not dispatched: %s", provider, cause)
}
