package ops

import (
	"sync"
	"time"
)

// Progress is the live state of a running job; safe for concurrent use.
type Progress struct {
	mu         sync.Mutex
	phase      string
	current    string
	filesDone  int
	filesTotal int
	bytesDone  int64
	bytesTotal int64
	committed  bool
	log        []string
	startedAt  time.Time
}

// Snapshot is the JSON view of Progress.
type Snapshot struct {
	Phase      string `json:"phase"`
	Current    string `json:"current"`
	FilesDone  int    `json:"filesDone"`
	FilesTotal int    `json:"filesTotal"`
	BytesDone  int64  `json:"bytesDone"`
	BytesTotal int64  `json:"bytesTotal"`
	// Committed turns true when the first destructive step starts; from then
	// on the job can no longer be cancelled.
	Committed bool     `json:"committed"`
	Log       []string `json:"log"`
	Seconds   float64  `json:"seconds"`
}

func newProgress(files int, bytes int64) *Progress {
	return &Progress{filesTotal: files, bytesTotal: bytes, startedAt: time.Now()}
}

func (p *Progress) setPhase(s string) {
	p.mu.Lock()
	p.phase = s
	p.log = append(p.log, s)
	p.mu.Unlock()
}

func (p *Progress) logf(s string) {
	p.mu.Lock()
	p.log = append(p.log, s)
	p.mu.Unlock()
}

func (p *Progress) setCurrent(s string) { p.mu.Lock(); p.current = s; p.mu.Unlock() }
func (p *Progress) addBytes(n int64)    { p.mu.Lock(); p.bytesDone += n; p.mu.Unlock() }
func (p *Progress) addFile()            { p.mu.Lock(); p.filesDone++; p.mu.Unlock() }
func (p *Progress) commit()             { p.mu.Lock(); p.committed = true; p.mu.Unlock() }

// Snapshot returns a copy of the state.
func (p *Progress) Snapshot() Snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	return Snapshot{
		Phase: p.phase, Current: p.current,
		FilesDone: p.filesDone, FilesTotal: p.filesTotal,
		BytesDone: p.bytesDone, BytesTotal: p.bytesTotal,
		Committed: p.committed, Log: append(make([]string, 0, len(p.log)), p.log...),
		Seconds: time.Since(p.startedAt).Seconds(),
	}
}
