// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package agent

import (
	"errors"
	"fmt"
	"sync"

	"github.com/alibaba/open-code-review/internal/llm"
	"github.com/alibaba/open-code-review/internal/session"
)

// runStop records the first provider error that makes every further request
// of the run fail. Dispatch stops on it; groups already in flight finish or
// fail on their own, so nothing that is still working gets cancelled.
type runStop struct {
	mu  sync.Mutex
	err error
}

// stopRunOn records err when it is fatal for the run and reports whether it
// was. Only the first fatal error is announced and stored.
func (a *Agent) stopRunOn(err error) bool {
	if err == nil || !errors.Is(err, llm.ErrFatalForRun) {
		return false
	}
	a.stop.mu.Lock()
	first := a.stop.err == nil
	if first {
		a.stop.err = err
	}
	a.stop.mu.Unlock()
	if !first {
		return true
	}

	a.recordWarning("run_stopped", "", fmt.Sprintf("%s: %v", a.stopProvider(), err))
	// A pending cause rather than a run failure: the files already reviewed
	// stay valid, so the terminal state follows coverage (partial, exit 3).
	if b := a.session.Manifest(); b != nil {
		if serr := b.SetPendingFailureCause(session.FailureProvider, "not reviewed: the provider stopped serving requests for this run"); serr != nil {
			a.recordWarning("manifest_error", "", serr.Error())
		}
	}
	return true
}

func (a *Agent) stopProvider() string {
	if a.args.Provider != "" {
		return a.args.Provider
	}
	return "the LLM provider"
}

// RunStopMessage describes why dispatch stopped early, or returns "" when it
// did not. The caller prints it once, next to the resume hint.
func (a *Agent) RunStopMessage() string {
	err := a.StoppedBy()
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%s stopped serving requests (%v); the remaining files were not dispatched", a.stopProvider(), err)
}

// StoppedBy returns the provider error that stopped dispatch early, or nil.
func (a *Agent) StoppedBy() error {
	a.stop.mu.Lock()
	defer a.stop.mu.Unlock()
	return a.stop.err
}
