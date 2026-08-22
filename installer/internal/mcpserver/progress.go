package mcpserver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/review"
)

const (
	progressDebounceInterval  = time.Second
	progressHeartbeatInterval = 15 * time.Second
)

type progressNotifyFunc func(context.Context, *mcp.ProgressNotificationParams) error

type progressEmitter struct {
	ctx    context.Context
	token  any
	total  int
	notify progressNotifyFunc
	now    func() time.Time

	debounce  time.Duration
	heartbeat time.Duration
	stop      chan struct{}
	done      chan struct{}
	reset     chan struct{}

	entryState func(string) review.State

	mu        sync.Mutex
	sendMu    sync.Mutex
	started   bool
	stopped   bool
	lastSent  time.Time
	states    map[string]review.State
	completed map[string]bool
}

func newReviewProgressEmitter(ctx context.Context, req *mcp.CallToolRequest, runDir string, entries []review.Entry) *progressEmitter {
	if req == nil || req.Params == nil || req.Session == nil {
		return nil
	}
	token := req.Params.GetProgressToken()
	if token == nil {
		return nil
	}
	emitter := newProgressEmitter(ctx, token, len(entries), req.Session.NotifyProgress, progressHeartbeatInterval, progressDebounceInterval, time.Now)
	emitter.entryState = func(entry string) review.State {
		return review.EntryState(runDir, entry)
	}
	emitter.start()
	return emitter
}

func newProgressEmitter(ctx context.Context, token any, total int, notify progressNotifyFunc, heartbeat time.Duration, debounce time.Duration, now func() time.Time) *progressEmitter {
	if token == nil || notify == nil {
		return nil
	}
	if now == nil {
		now = time.Now
	}
	emitter := &progressEmitter{
		ctx:       ctx,
		token:     token,
		total:     total,
		notify:    notify,
		now:       now,
		debounce:  debounce,
		heartbeat: heartbeat,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
		reset:     make(chan struct{}, 1),
		states:    map[string]review.State{},
		completed: map[string]bool{},
	}
	return emitter
}

func (e *progressEmitter) start() {
	if e == nil || e.heartbeat <= 0 {
		return
	}
	e.mu.Lock()
	if e.started || e.stopped {
		e.mu.Unlock()
		return
	}
	e.started = true
	e.mu.Unlock()
	go e.runHeartbeat()
}

func (e *progressEmitter) runHeartbeat() {
	defer close(e.done)
	timer := time.NewTimer(e.heartbeat)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			e.emit("scan running", false)
			resetTimer(timer, e.heartbeat)
		case <-e.reset:
			resetTimer(timer, e.heartbeat)
		case <-e.stop:
			return
		}
	}
}

func resetTimer(timer *time.Timer, duration time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(duration)
}

func (e *progressEmitter) Report(entry string, job review.JobStatus) {
	if e == nil {
		return
	}
	message := e.record(entry, job)
	e.emit(message, false)
}

func (e *progressEmitter) Stop(message string) {
	if e == nil {
		return
	}
	e.sendMu.Lock()
	params := e.stopParams(message)
	if params != nil {
		_ = e.notify(e.ctx, params)
	}
	e.sendMu.Unlock()
	if e.isStarted() {
		<-e.done
	}
}

func (e *progressEmitter) record(entry string, job review.JobStatus) string {
	state := job.State
	if e.entryState != nil {
		state = e.entryState(entry)
		if state == "" {
			state = job.State
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopped {
		return ""
	}
	e.states[entry] = state
	if terminalReviewState(state) {
		e.completed[entry] = true
	} else {
		delete(e.completed, entry)
	}
	return fmt.Sprintf("%s %s", entry, progressStateVerb(state))
}

func (e *progressEmitter) isStarted() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.started
}

func (e *progressEmitter) emit(message string, force bool) {
	if e == nil || message == "" {
		return
	}
	e.sendMu.Lock()
	defer e.sendMu.Unlock()

	params := e.params(message, force)
	if params == nil {
		return
	}
	_ = e.notify(e.ctx, params)
	e.resetHeartbeat()
}

func (e *progressEmitter) params(message string, force bool) *mcp.ProgressNotificationParams {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopped {
		return nil
	}
	now := e.now()
	if !force && !e.lastSent.IsZero() && e.debounce > 0 && now.Sub(e.lastSent) < e.debounce {
		return nil
	}
	e.lastSent = now
	completed, running := e.countsLocked()
	return &mcp.ProgressNotificationParams{
		ProgressToken: e.token,
		Progress:      float64(completed),
		Total:         float64(e.total),
		Message:       progressMessage(message, completed, running, e.total),
	}
}

func (e *progressEmitter) stopParams(message string) *mcp.ProgressNotificationParams {
	e.mu.Lock()
	if e.stopped {
		e.mu.Unlock()
		return nil
	}
	e.stopped = true
	now := e.now()
	if !e.lastSent.IsZero() && e.debounce > 0 && now.Sub(e.lastSent) < e.debounce {
		close(e.stop)
		e.mu.Unlock()
		return nil
	}
	e.lastSent = now
	completed, running := e.countsLocked()
	params := &mcp.ProgressNotificationParams{
		ProgressToken: e.token,
		Progress:      float64(completed),
		Total:         float64(e.total),
		Message:       progressMessage(message, completed, running, e.total),
	}
	close(e.stop)
	e.mu.Unlock()
	return params
}

func (e *progressEmitter) resetHeartbeat() {
	if e.heartbeat <= 0 {
		return
	}
	select {
	case e.reset <- struct{}{}:
	default:
	}
}

func (e *progressEmitter) countsLocked() (int, int) {
	completed := len(e.completed)
	running := 0
	for entry, state := range e.states {
		if e.completed[entry] {
			continue
		}
		if state == review.StateRunning {
			running++
		}
	}
	return completed, running
}

func progressMessage(message string, completed int, running int, total int) string {
	if total <= 0 {
		return message
	}
	if running > 0 {
		return fmt.Sprintf("%s (%d/%d complete, %d running)", message, completed, total, running)
	}
	return fmt.Sprintf("%s (%d/%d complete)", message, completed, total)
}

func progressStateVerb(state review.State) string {
	switch state {
	case review.StateRunning:
		return "running"
	case review.StateDone:
		return "finished"
	case review.StateFailed:
		return "failed"
	case review.StateSkipped:
		return "skipped"
	default:
		return string(state)
	}
}

func terminalReviewState(state review.State) bool {
	return state == review.StateDone || state == review.StateFailed || state == review.StateSkipped
}
