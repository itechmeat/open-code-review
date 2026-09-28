// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

//go:build !windows

package llm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestClaudeCodeClientCancelKillsProcessGroup(t *testing.T) {
	useFakeClaude(t, "spawn-sleep")
	pidfile := filepath.Join(t.TempDir(), "child.pid")
	t.Setenv("OCR_FAKE_CLAUDE_PIDFILE", pidfile)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for ctx.Err() == nil {
			if _, err := os.Stat(pidfile); err == nil {
				cancel()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	start := time.Now()
	_, err := NewClaudeCodeClient(ClientConfig{Model: "haiku"}).CompletionsWithCtx(ctx, toolRequest())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	// A child that survived would hold the output pipe until WaitDelay (5 s).
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("cancel took %v", elapsed)
	}

	data, err := os.ReadFile(pidfile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	// The orphaned child is reaped by init shortly after the kill.
	deadline := time.Now().Add(2 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("child process %d survived the cancel", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
