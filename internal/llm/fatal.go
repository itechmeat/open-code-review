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
