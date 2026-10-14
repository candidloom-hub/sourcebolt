// Package jobs is the in-process table behind long-running tool calls
// (deep research, async chat). Jobs live in memory only.
package jobs

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// Job statuses.
const (
	StatusRunning   = "running"
	StatusDone      = "done"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// retention is how long a finished job stays readable.
const retention = time.Hour

// Job is a snapshot of one table entry.
type Job struct {
	ID, Kind, Status string
	Result           any
	Err              error
	Started          time.Time
}

type entry struct {
	job      Job
	cancel   context.CancelFunc
	finished time.Time // zero while running
}

// Table holds jobs. The zero value is not usable; call New.
type Table struct {
	mu   sync.Mutex
	now  func() time.Time
	jobs map[string]*entry
}

// Option configures New.
type Option func(*Table)

// WithClock replaces time.Now, for tests.
func WithClock(now func() time.Time) Option { return func(t *Table) { t.now = now } }

// New returns an empty table.
func New(opts ...Option) *Table {
	t := &Table{now: time.Now, jobs: map[string]*entry{}}
	for _, o := range opts {
		o(t)
	}
	return t
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("jobs: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}

// sweep drops finished jobs older than retention. Caller holds t.mu.
func (t *Table) sweep() {
	now := t.now()
	for id, e := range t.jobs {
		if !e.finished.IsZero() && now.Sub(e.finished) >= retention {
			delete(t.jobs, id)
		}
	}
}

// Start runs f in a new goroutine and returns the job id. f's context is
// cancelled by Cancel. A panic in f marks the job failed.
func (t *Table) Start(kind string, f func(ctx context.Context) (any, error)) string {
	ctx, cancel := context.WithCancel(context.Background())
	id := newID()

	t.mu.Lock()
	t.sweep()
	for t.jobs[id] != nil { // astronomically unlikely; keep ids unique anyway
		id = newID()
	}
	e := &entry{cancel: cancel, job: Job{ID: id, Kind: kind, Status: StatusRunning, Started: t.now()}}
	t.jobs[id] = e
	t.mu.Unlock()

	go func() {
		var (
			res any
			err error
		)
		func() {
			defer func() {
				if r := recover(); r != nil {
					res, err = nil, fmt.Errorf("job panicked: %v", r)
				}
			}()
			res, err = f(ctx)
		}()
		cancel() // release the context's resources

		t.mu.Lock()
		defer t.mu.Unlock()
		if e.job.Status != StatusRunning { // cancelled meanwhile; keep that
			return
		}
		e.finished = t.now()
		if err != nil {
			e.job.Status, e.job.Err = StatusFailed, err
			return
		}
		e.job.Status, e.job.Result = StatusDone, res
	}()
	return id
}

// Get returns a snapshot of the job, or false if unknown or evicted.
func (t *Table) Get(id string) (Job, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sweep()
	e, ok := t.jobs[id]
	if !ok {
		return Job{}, false
	}
	return e.job, true
}

// Cancel cancels a running job and marks it cancelled at once; it reports
// whether the job was running.
func (t *Table) Cancel(id string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sweep()
	e, ok := t.jobs[id]
	if !ok || e.job.Status != StatusRunning {
		return false
	}
	e.job.Status = StatusCancelled
	e.finished = t.now()
	e.cancel()
	return true
}
