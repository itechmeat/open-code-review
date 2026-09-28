// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package scan

import "github.com/alibaba/open-code-review/internal/llm"

// stopRunOn records err when it is fatal for the run and reports whether it
// was. Dispatch stops on it; files already in flight finish or fail on their
// own. Only the first fatal error is announced.
func (a *Agent) stopRunOn(err error) bool {
	fatal, first := a.stop.Record(err)
	if first {
		a.recordWarning("run_stopped", "", a.RunStopMessage())
	}
	return fatal
}

// RunStopMessage describes why dispatch stopped early, or returns "" when it
// did not.
func (a *Agent) RunStopMessage() string {
	return llm.RunStopMessage(a.args.Provider, a.StoppedBy())
}

// StoppedBy returns the provider error that stopped dispatch early, or nil.
func (a *Agent) StoppedBy() error { return a.stop.Err() }
