// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package agent

import (
	"github.com/alibaba/open-code-review/internal/llm"
	"github.com/alibaba/open-code-review/internal/session"
)

// stopRunOn records err when it is fatal for the run and reports whether it
// was. Dispatch stops on it; groups already in flight finish or fail on their
// own, so nothing that is still working gets cancelled. Only the first fatal
// error is announced.
func (a *Agent) stopRunOn(err error) bool {
	fatal, first := a.stop.Record(err)
	if !first {
		return fatal
	}

	a.recordWarning("run_stopped", "", a.RunStopMessage())
	// A pending cause rather than a run failure: the files already reviewed
	// stay valid, so the terminal state follows coverage (partial, exit 3).
	if b := a.session.Manifest(); b != nil {
		if serr := b.SetPendingFailureCause(session.FailureProvider, "not reviewed: the provider stopped serving requests for this run"); serr != nil {
			a.recordWarning("manifest_error", "", serr.Error())
		}
	}
	return true
}

// RunStopMessage describes why dispatch stopped early, or returns "" when it
// did not. The caller prints it once, next to the resume hint.
func (a *Agent) RunStopMessage() string {
	return llm.RunStopMessage(a.args.Provider, a.StoppedBy())
}

// StoppedBy returns the provider error that stopped dispatch early, or nil.
func (a *Agent) StoppedBy() error { return a.stop.Err() }
