// Package jobs is the in-process table behind long-running tool calls
// (deep research, async chat). Jobs live in memory only.
package jobs

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
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

// Errors returned by Submit.
var (
	ErrBusy   = errors.New("jobs: too many running jobs of this kind")
	ErrClosed = errors.New("jobs: table closed")
)

// Job is a snapshot of one table entry.
type Job struct {
	ID, Kind, Status string
	Result           any
	Err              error
	Started          time.Time
	// Meta is caller data attached at Submit (for example a conversation
	// id). It is dropped when the job finishes or is cancelled. Treat it
	// as read-only.
	Meta map[string]string
}

type entry struct {
	job      Job
	cancel   context.CancelFunc
	finished time.Time // zero while running
}

// Table holds jobs. The zero value is not usable; call New.
type Table struct {
	mu     sync.Mutex
	now    func() time.Time
	jobs   map[string]*entry
	base   context.Context // parent of every job context; ended by Close
	stop   context.CancelFunc
	closed bool
}

// Option configures New.
type Option func(*Table)

// WithClock replaces time.Now, for tests.
func WithClock(now func() time.Time) Option { return func(t *Table) { t.now = now } }

// New returns an empty table. Call Close when the server stops.
func New(opts ...Option) *Table {
	base, stop := context.WithCancel(context.Background())
	t := &Table{now: time.Now, jobs: map[string]*entry{}, base: base, stop: stop}
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

// StartOption configures Submit.
type StartOption func(*startConfig)

type startConfig struct {
	limit int
	meta  map[string]string
}

// Limit refuses the job (ErrBusy) when n jobs of the same kind are
// already running. n <= 0 means no limit.
func Limit(n int) StartOption { return func(c *startConfig) { c.limit = n } }

// Meta attaches a key/value to the job; see Job.Meta.
func Meta(key, value string) StartOption {
	return func(c *startConfig) {
		if c.meta == nil {
			c.meta = map[string]string{}
		}
		c.meta[key] = value
	}
}

// Start runs f in a new goroutine and returns the job id ("" when the
// table is closed). See Submit.
func (t *Table) Start(kind string, f func(ctx context.Context) (any, error)) string {
	id, _ := t.Submit(kind, f)
	return id
}

// Submit runs f in a new goroutine and returns the job id. f's context is
// cancelled by Cancel and by Close. A panic in f marks the job failed.
func (t *Table) Submit(kind string, f func(ctx context.Context) (any, error), opts ...StartOption) (string, error) {
	var cfg startConfig
	for _, o := range opts {
		o(&cfg)
	}

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return "", ErrClosed
	}
	t.sweep()
	if cfg.limit > 0 {
		running := 0
		for _, e := range t.jobs {
			if e.job.Kind == kind && e.job.Status == StatusRunning {
				running++
			}
		}
		if running >= cfg.limit {
			t.mu.Unlock()
			return "", fmt.Errorf("%w (%d %s jobs running)", ErrBusy, running, kind)
		}
	}
	id := newID()
	for t.jobs[id] != nil { // astronomically unlikely; keep ids unique anyway
		id = newID()
	}
	ctx, cancel := context.WithCancel(t.base)
	e := &entry{cancel: cancel, job: Job{ID: id, Kind: kind, Status: StatusRunning, Started: t.now(), Meta: cfg.meta}}
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
		e.job.Meta = nil
		if err != nil {
			e.job.Status, e.job.Err = StatusFailed, err
			return
		}
		e.job.Status, e.job.Result = StatusDone, res
	}()
	return id, nil
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
	t.cancelLocked(e)
	return true
}

// cancelLocked marks e cancelled and ends its context. Caller holds t.mu.
func (t *Table) cancelLocked(e *entry) {
	e.job.Status = StatusCancelled
	e.job.Meta = nil
	e.finished = t.now()
	e.cancel()
}

// Close cancels every running job and refuses new ones. Finished jobs stay
// readable. It is idempotent.
func (t *Table) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	t.closed = true
	for _, e := range t.jobs {
		if e.job.Status == StatusRunning {
			t.cancelLocked(e)
		}
	}
	t.stop()
}
