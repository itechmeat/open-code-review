// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func withRateLimitDelays(t *testing.T, base, maxDelay time.Duration) {
	t.Helper()
	oldBase, oldMax := rateLimitBaseDelay, rateLimitMaxDelay
	rateLimitBaseDelay, rateLimitMaxDelay = base, maxDelay
	t.Cleanup(func() { rateLimitBaseDelay, rateLimitMaxDelay = oldBase, oldMax })
}

func quietLimiter() *adaptiveLimiter {
	l := newAdaptiveLimiter()
	l.logf = func(string, ...any) {}
	return l
}

func TestRateLimitDelayGrowsWithJitterAndCap(t *testing.T) {
	withRateLimitDelays(t, 2*time.Second, 60*time.Second)
	for retry, want := range []time.Duration{2, 4, 8, 16, 32, 60, 60} {
		want *= time.Second
		for i := 0; i < 50; i++ {
			got := rateLimitDelay(retry)
			if got < want/2 || got > want {
				t.Fatalf("retry %d: delay %v outside [%v, %v]", retry, got, want/2, want)
			}
		}
	}
	if got := rateLimitDelay(1000); got > 60*time.Second || got < 30*time.Second {
		t.Fatalf("huge retry count: delay %v not capped", got)
	}
	if got := rateLimitDelay(-1); got > 2*time.Second {
		t.Fatalf("negative retry count: delay %v", got)
	}
}

func runMiddleware(t *testing.T, mw retryObserver, req *http.Request, res *http.Response) *http.Response {
	t.Helper()
	got, err := mw(req, func(*http.Request) (*http.Response, error) { return res, nil })
	if err != nil {
		t.Fatalf("middleware: %v", err)
	}
	return got
}

func TestRateLimitMiddlewareInjectsHintOnlyWhenMissing(t *testing.T) {
	withRateLimitDelays(t, 100*time.Millisecond, time.Second)
	mw := newRateLimitMiddleware(quietLimiter())

	req := httptest.NewRequest(http.MethodPost, "http://x/", nil)
	req.Header.Set("X-Stainless-Retry-Count", "2")
	res := runMiddleware(t, mw, req, &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}, Body: http.NoBody})
	ms, err := strconv.Atoi(res.Header.Get("Retry-After-Ms"))
	if err != nil {
		t.Fatalf("Retry-After-Ms not injected: %q", res.Header.Get("Retry-After-Ms"))
	}
	if ms < 200 || ms > 400 {
		t.Fatalf("Retry-After-Ms = %d, want within [200, 400] for the third attempt", ms)
	}

	for _, h := range []string{"Retry-After", "Retry-After-Ms"} {
		hdr := http.Header{}
		hdr.Set(h, "7")
		res := runMiddleware(t, mw, httptest.NewRequest(http.MethodPost, "http://x/", nil),
			&http.Response{StatusCode: http.StatusTooManyRequests, Header: hdr, Body: http.NoBody})
		if h == "Retry-After" && res.Header.Get("Retry-After-Ms") != "" {
			t.Fatalf("server Retry-After must win, got injected Retry-After-Ms %q", res.Header.Get("Retry-After-Ms"))
		}
		if res.Header.Get(h) != "7" {
			t.Fatalf("server %s rewritten to %q", h, res.Header.Get(h))
		}
	}

	res = runMiddleware(t, mw, httptest.NewRequest(http.MethodPost, "http://x/", nil),
		&http.Response{StatusCode: http.StatusServiceUnavailable, Header: http.Header{}, Body: http.NoBody})
	if res.Header.Get("Retry-After-Ms") != "" {
		t.Fatal("non-429 responses must keep the SDK's default backoff")
	}
}

func TestAdaptiveLimiterHalvesOncePerEpisodeAndRecovers(t *testing.T) {
	l := quietLimiter()
	ctx := context.Background()

	var releases []func(attemptOutcome)
	for i := 0; i < 8; i++ {
		r, err := l.acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, r)
	}
	// All eight were in flight together, so their 429s are one episode.
	for _, r := range releases {
		r(outcomeRateLimited)
	}
	if got := l.currentLimit(); got != 4 {
		t.Fatalf("limit after one episode = %d, want 4", got)
	}

	// A new episode, started after the decrease, halves again.
	r, _ := l.acquire(ctx)
	r(outcomeRateLimited)
	if got := l.currentLimit(); got != 2 {
		t.Fatalf("limit after second episode = %d, want 2", got)
	}

	// Additive increase: the cap lifts once the limit is back at the ceiling.
	for i := 0; i < 100 && l.currentLimit() != 0; i++ {
		r, _ := l.acquire(ctx)
		r(outcomeSuccess)
	}
	if got := l.currentLimit(); got != 0 {
		t.Fatalf("limit did not recover, still %d", got)
	}
}

func TestAdaptiveLimiterBlocksAboveLimitAndHonorsContext(t *testing.T) {
	l := quietLimiter()
	first, _ := l.acquire(context.Background())
	second, _ := l.acquire(context.Background())
	second(outcomeRateLimited) // in flight was 2 -> limit 1
	if got := l.currentLimit(); got != 1 {
		t.Fatalf("limit = %d, want 1", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := l.acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("acquire above limit: err = %v, want deadline exceeded", err)
	}

	admitted := make(chan struct{})
	go func() {
		r, err := l.acquire(context.Background())
		if err == nil {
			r(outcomeOther)
		}
		close(admitted)
	}()
	select {
	case <-admitted:
		t.Fatal("waiter admitted while the only slot is taken")
	case <-time.After(20 * time.Millisecond):
	}
	first(outcomeOther)
	select {
	case <-admitted:
	case <-time.After(2 * time.Second):
		t.Fatal("waiter not woken after release")
	}
}

func TestRateLimitMiddlewareHoldsSlotUntilBodyDone(t *testing.T) {
	l := quietLimiter()
	mw := newRateLimitMiddleware(l)
	l.limit, l.ceiling = 1, 8

	res := runMiddleware(t, mw, httptest.NewRequest(http.MethodPost, "http://x/", nil),
		&http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("data"))})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := l.acquire(ctx); err == nil {
		t.Fatal("slot released before the body was consumed")
	}
	if _, err := io.ReadAll(res.Body); err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	r, err := l.acquire(context.Background())
	if err != nil {
		t.Fatalf("slot not released after EOF: %v", err)
	}
	r(outcomeOther)
}

// End to end through the OpenAI SDK: a burst of hint-less 429s is retried with
// the stretched backoff and the request still succeeds.
func TestOpenAIClientRecoversFromHintless429(t *testing.T) {
	withRateLimitDelays(t, 20*time.Millisecond, 100*time.Millisecond)
	var calls atomic.Int32
	var mu sync.Mutex
	var stamps []time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		stamps = append(stamps, time.Now())
		mu.Unlock()
		if calls.Add(1) <= 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"code":"1302","message":"rate limited"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	c := NewOpenAIClient(ClientConfig{URL: srv.URL, APIKey: "k", Model: "m"})
	resp, err := c.CompletionsWithCtx(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("completion failed: %v", err)
	}
	if resp == nil || calls.Load() != 4 {
		t.Fatalf("calls = %d, want 4", calls.Load())
	}
	mu.Lock()
	defer mu.Unlock()
	// Third retry waits at least half of base*2^2.
	if gap := stamps[3].Sub(stamps[2]); gap < 40*time.Millisecond {
		t.Fatalf("gap before 4th attempt = %v, want the stretched backoff (>= 40ms)", gap)
	}
}
