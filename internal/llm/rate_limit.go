// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package llm

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/alibaba/open-code-review/internal/stdout"
)

// The SDKs back off 0.5s..8s between retries, which is tuned for transient 5xx
// errors. A provider that answers 429 without a Retry-After hint (the Zhipu
// coding plan's code 1302, for one) usually needs tens of seconds, so five SDK
// retries inside ~15s all land in the same throttling window and the request is
// lost. These bounds stretch only that case.
var (
	rateLimitBaseDelay = 2 * time.Second
	rateLimitMaxDelay  = 60 * time.Second
)

// rateLimitDelay is exponential backoff with jitter for the retryCount-th retry
// of a rate-limited request: base*2^retryCount capped at max, then drawn from
// [d/2, d] so parallel requests throttled together do not retry together.
func rateLimitDelay(retryCount int) time.Duration {
	if retryCount < 0 {
		retryCount = 0
	}
	d := rateLimitMaxDelay
	if retryCount < 16 {
		if exp := rateLimitBaseDelay << retryCount; exp > 0 && exp < rateLimitMaxDelay {
			d = exp
		}
	}
	half := d / 2
	return half + time.Duration(rand.Int64N(int64(half)+1))
}

// hasRetryAfterHint reports whether the response already carries a server wait
// hint the SDKs honor. Those hints always win: the server knows its window.
func hasRetryAfterHint(h http.Header) bool {
	return h.Get("Retry-After-Ms") != "" || h.Get("Retry-After") != ""
}

// newRateLimitMiddleware returns the outermost transport middleware. It
//
//   - gates every HTTP attempt through the client's adaptive limiter, which
//     lowers parallelism after 429s and restores it as requests succeed;
//   - gives a 429 that carries no Retry-After hint a Retry-After-Ms computed by
//     rateLimitDelay, so the SDK's own retry loop waits long enough.
//
// It is mounted outside the retry observer, so the retry report still records
// exactly what the server sent, and the injected hint only steers the SDK.
func newRateLimitMiddleware(l *adaptiveLimiter) retryObserver {
	return func(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
		release, err := l.acquire(req.Context())
		if err != nil {
			return nil, err
		}
		res, err := next(req)
		if err != nil || res == nil {
			release(outcomeOther)
			return res, err
		}
		if res.StatusCode == http.StatusTooManyRequests {
			if !hasRetryAfterHint(res.Header) {
				retryCount, _ := strconv.Atoi(req.Header.Get("X-Stainless-Retry-Count"))
				res.Header.Set("Retry-After-Ms", strconv.FormatInt(rateLimitDelay(retryCount).Milliseconds(), 10))
			}
			release(outcomeRateLimited)
			return res, nil
		}
		outcome := outcomeOther
		if res.StatusCode >= 200 && res.StatusCode < 300 {
			outcome = outcomeSuccess
		}
		if res.Body == nil {
			release(outcome)
			return res, nil
		}
		// A streamed body is still being produced by the provider, so the slot is
		// held until the SDK finishes with it. The context hook frees it if the
		// body is abandoned without Close.
		once := &sync.Once{}
		done := func() { once.Do(func() { release(outcome) }) }
		stop := context.AfterFunc(req.Context(), done)
		res.Body = &releasingBody{ReadCloser: res.Body, done: func() { stop(); done() }}
		return res, nil
	}
}

type releasingBody struct {
	io.ReadCloser
	done func()
}

func (b *releasingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err == io.EOF {
		b.done()
	}
	return n, err
}

func (b *releasingBody) Close() error {
	err := b.ReadCloser.Close()
	b.done()
	return err
}

type attemptOutcome int

const (
	outcomeSuccess attemptOutcome = iota
	outcomeRateLimited
	outcomeOther
)

// adaptiveLimiter caps in-flight HTTP attempts with AIMD: unlimited until the
// provider first answers 429, then halved on each new throttling episode and
// raised by one slot per window of successes, until it is back at the level
// that first triggered throttling and the cap is lifted again.
//
// An episode is identified by epoch: a 429 only lowers the limit when its
// attempt started after the previous decrease, so a burst of 429s from requests
// that were all in flight together counts once instead of collapsing the limit
// to one.
type adaptiveLimiter struct {
	mu       sync.Mutex
	limit    float64 // 0 = no cap
	ceiling  int     // in-flight level that first hit 429; the cap lifts there
	inFlight int
	epoch    uint64
	waiters  []chan struct{}
	logf     func(format string, args ...any)
}

func newAdaptiveLimiter() *adaptiveLimiter {
	return &adaptiveLimiter{logf: func(format string, args ...any) {
		fmt.Fprintf(stdout.Writer(), format, args...)
	}}
}

// acquire blocks until the limiter admits another attempt or ctx ends. The
// returned release must be called exactly once with the attempt's outcome.
func (l *adaptiveLimiter) acquire(ctx context.Context) (func(attemptOutcome), error) {
	for {
		l.mu.Lock()
		if l.limit == 0 || l.inFlight < int(l.limit) {
			l.inFlight++
			epoch := l.epoch
			l.mu.Unlock()
			return func(o attemptOutcome) { l.release(epoch, o) }, nil
		}
		ch := make(chan struct{})
		l.waiters = append(l.waiters, ch)
		l.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			l.mu.Lock()
			l.dropWaiter(ch)
			l.mu.Unlock()
			return nil, ctx.Err()
		}
	}
}

func (l *adaptiveLimiter) release(epoch uint64, o attemptOutcome) {
	l.mu.Lock()
	defer l.mu.Unlock()
	inFlightBefore := l.inFlight
	l.inFlight--
	switch o {
	case outcomeRateLimited:
		if epoch != l.epoch {
			break
		}
		prev := l.limit
		if l.limit == 0 {
			l.ceiling = inFlightBefore
			l.limit = float64(inFlightBefore)
		}
		l.limit = max(1, float64(int(l.limit/2)))
		l.epoch++
		if int(l.limit) != int(prev) {
			l.logf("[ocr] Provider rate limit hit; lowering concurrent LLM requests to %d\n", int(l.limit))
		}
	case outcomeSuccess:
		if l.limit == 0 {
			break
		}
		l.limit += 1 / l.limit
		if int(l.limit) >= l.ceiling {
			l.limit = 0
			l.logf("[ocr] Provider rate limit cleared; concurrent LLM requests no longer capped\n")
		}
	}
	l.wakeLocked()
}

// wakeLocked wakes as many waiters as there are free slots. Woken waiters
// re-check the limit themselves, so waking one too many is harmless.
func (l *adaptiveLimiter) wakeLocked() {
	free := len(l.waiters)
	if l.limit != 0 {
		free = int(l.limit) - l.inFlight
	}
	for free > 0 && len(l.waiters) > 0 {
		close(l.waiters[0])
		l.waiters = l.waiters[1:]
		free--
	}
}

func (l *adaptiveLimiter) dropWaiter(ch chan struct{}) {
	for i, w := range l.waiters {
		if w == ch {
			l.waiters = append(l.waiters[:i], l.waiters[i+1:]...)
			return
		}
	}
	// Already woken: pass the wakeup on so the slot is not lost.
	l.wakeLocked()
}

// currentLimit reports the cap (0 = none); for tests and diagnostics.
func (l *adaptiveLimiter) currentLimit() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return int(l.limit)
}
