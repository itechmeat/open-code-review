// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// claudeCodeThread is one OCR conversation (ChatRequest.SessionID) carried by
// one persisted Claude Code session. Resuming it sends only the messages added
// since the last call instead of replaying the whole history, which on a
// measured main-task loop cut uncached input per follow-up round about 5-9x.
type claudeCodeThread struct {
	sessionID string
	covered   int    // len(Messages) of the last request the session answered
	digest    string // claudeCodeDigest of those messages
}

func (c *ClaudeCodeClient) completeInThread(ctx context.Context, bin string, req ChatRequest,
	inv claudeCodeInvocation, model string) (*ChatResponse, error) {
	dir, err := c.sessionDir()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	th := c.threads[req.SessionID]
	c.mu.Unlock()

	// A thread resumes only while OCR's history still starts with what the
	// session saw; memory compression or a new review round rewrites it.
	if th != nil && len(req.Messages) > th.covered && claudeCodeDigest(inv.SystemPrompt, req.Messages[:th.covered]) == th.digest {
		resp, err := c.run(ctx, bin, dir, append(inv.Args, "--resume", th.sessionID),
			renderClaudeCodeDelta(req.Messages, th.covered), inv, model)
		if err == nil {
			c.remember(req, inv, th.sessionID)
			return resp, nil
		}
		if ctx.Err() != nil || errors.Is(err, ErrClaudeCodeNotLoggedIn) ||
			errors.Is(err, ErrClaudeCodeUsageLimit) || errors.Is(err, ErrClaudeCodeOutdated) {
			return nil, err
		}
		// Anything else (a lost or unreadable session) gets one full retry.
	}

	sessionID, err := newClaudeCodeSessionID()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.sessions = append(c.sessions, sessionID)
	c.mu.Unlock()
	resp, err := c.run(ctx, bin, dir, append(inv.Args, "--session-id", sessionID), inv.Stdin, inv, model)
	if err != nil {
		c.mu.Lock()
		delete(c.threads, req.SessionID)
		c.mu.Unlock()
		return nil, err
	}
	c.remember(req, inv, sessionID)
	return resp, nil
}

func (c *ClaudeCodeClient) remember(req ChatRequest, inv claudeCodeInvocation, sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.threads == nil {
		c.threads = make(map[string]*claudeCodeThread)
	}
	c.threads[req.SessionID] = &claudeCodeThread{
		sessionID: sessionID,
		covered:   len(req.Messages),
		digest:    claudeCodeDigest(inv.SystemPrompt, req.Messages),
	}
}

func (c *ClaudeCodeClient) sessionDir() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.workDir == "" {
		dir, err := os.MkdirTemp("", "ocr-claude-*")
		if err != nil {
			return "", fmt.Errorf("claude-code: create working directory: %w", err)
		}
		c.workDir = dir
	}
	return c.workDir, nil
}

// Close deletes the working directory and the transcripts of every session
// this client started, so reviews leave nothing in ~/.claude/projects.
func (c *ClaudeCodeClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	base := os.Getenv("CLAUDE_CONFIG_DIR")
	if base == "" {
		if home, err := os.UserHomeDir(); err == nil {
			base = filepath.Join(home, ".claude")
		}
	}
	var errs []error
	if base != "" {
		for _, id := range c.sessions {
			matches, _ := filepath.Glob(filepath.Join(base, "projects", "*", id+".jsonl"))
			for _, m := range matches {
				if err := os.Remove(m); err != nil && !os.IsNotExist(err) {
					errs = append(errs, err)
				}
				_ = os.RemoveAll(filepath.Join(filepath.Dir(m), id))
				// Succeeds only once the project directory is empty.
				_ = os.Remove(filepath.Dir(m))
			}
		}
	}
	if c.workDir != "" {
		if err := os.RemoveAll(c.workDir); err != nil {
			errs = append(errs, err)
		}
	}
	c.sessions, c.threads, c.workDir = nil, nil, ""
	return errors.Join(errs...)
}

func claudeCodeDigest(systemPrompt string, msgs []Message) string {
	h := sha256.New()
	h.Write([]byte(systemPrompt))
	h.Write([]byte{0})
	h.Write([]byte(renderClaudeCodeTranscript(msgs)))
	return hex.EncodeToString(h.Sum(nil))
}

func newClaudeCodeSessionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("claude-code: session id: %w", err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
