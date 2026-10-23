package ops

import (
	"context"
	"errors"
	"sync"
	"time"

	"afvi/tools/core/internal/i18n"
)

// Job states.
const (
	StateRunning  = "running"
	StateDone     = "done"
	StateFailed   = "failed"
	StateCanceled = "canceled"
)

// JobView is the JSON view of a job.
type JobView struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	State    string   `json:"state"`
	Error    string   `json:"error,omitempty"`
	Progress Snapshot `json:"progress"`
}

type job struct {
	id     string
	title  string
	prog   *Progress
	cancel context.CancelFunc
	mu     sync.Mutex
	state  string
	err    string
}

func (j *job) view() JobView {
	j.mu.Lock()
	defer j.mu.Unlock()
	return JobView{ID: j.id, Title: j.title, State: j.state, Error: j.err, Progress: j.prog.Snapshot()}
}

const planTTL = 30 * time.Minute

type storedPlan struct {
	plan *Plan
	at   time.Time
}

// Manager keeps plans waiting for confirmation and the jobs started from
// them. Only one job runs at a time: two jobs on the same shares would only
// get in each other's way.
type Manager struct {
	mu      sync.Mutex
	plans   map[string]storedPlan
	jobs    map[string]*job
	running *job
}

// NewManager returns an empty manager.
func NewManager() *Manager {
	return &Manager{plans: map[string]storedPlan{}, jobs: map[string]*job{}}
}

// Add stores a plan until it is started or expires.
func (m *Manager) Add(p *Plan) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, sp := range m.plans {
		if time.Since(sp.at) > planTTL {
			delete(m.plans, id)
		}
	}
	m.plans[p.ID] = storedPlan{plan: p, at: time.Now()}
}

// ErrBusy is returned by Start while another job runs.
var ErrBusy = errors.New("another job is already running")

// ErrNoPlan is returned by Start for an unknown or expired plan.
var ErrNoPlan = errors.New("plan not found or expired")

// Start runs a stored plan in the background. A plan can be started once.
func (m *Manager) Start(planID string) (JobView, error) {
	m.mu.Lock()
	if m.running != nil {
		m.mu.Unlock()
		return JobView{}, ErrBusy
	}
	sp, ok := m.plans[planID]
	if !ok || time.Since(sp.at) > planTTL {
		m.mu.Unlock()
		return JobView{}, ErrNoPlan
	}
	if len(sp.plan.Blockers) > 0 {
		m.mu.Unlock()
		return JobView{}, errors.New(i18n.For(sp.plan.lang)("j.blocked", sp.plan.Blockers[0]))
	}
	delete(m.plans, planID)

	files := sp.plan.Files
	if sp.plan.Kind == "rename" {
		files = len(sp.plan.units)
	}
	ctx, cancel := context.WithCancel(context.Background())
	j := &job{id: newID(), title: sp.plan.Title, prog: newProgress(files, sp.plan.Bytes), cancel: cancel, state: StateRunning}
	m.jobs[j.id] = j
	m.running = j
	m.mu.Unlock()

	go func() {
		err := Run(ctx, sp.plan, j.prog)
		j.mu.Lock()
		switch {
		case err == nil:
			j.state = StateDone
		case errors.Is(err, context.Canceled):
			j.state = StateCanceled
			j.err = i18n.For(sp.plan.lang)("j.canceled")
		default:
			j.state = StateFailed
			j.err = err.Error()
		}
		j.mu.Unlock()
		cancel()
		m.mu.Lock()
		m.running = nil
		m.mu.Unlock()
	}()
	return j.view(), nil
}

// Get returns the job view.
func (m *Manager) Get(id string) (JobView, bool) {
	m.mu.Lock()
	j, ok := m.jobs[id]
	m.mu.Unlock()
	if !ok {
		return JobView{}, false
	}
	return j.view(), true
}

// Cancel asks a job to stop. It only works before the first destructive
// step; afterwards it returns false.
func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	j, ok := m.jobs[id]
	m.mu.Unlock()
	if !ok {
		return false
	}
	if j.prog.Snapshot().Committed {
		return false
	}
	j.cancel()
	return true
}

// Idle reports whether no job runs.
func (m *Manager) Idle() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running == nil
}

// ProgressForTest returns a fresh Progress for tests that call Run directly.
func (m *Manager) ProgressForTest() *Progress { return newProgress(0, 0) }
