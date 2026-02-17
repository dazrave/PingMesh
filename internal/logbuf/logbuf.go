package logbuf

import (
	"bytes"
	"sync"
	"time"
)

const defaultCapacity = 1000

// Entry is a single log line with timestamp.
type Entry struct {
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
}

// Buffer is a thread-safe ring buffer of log entries.
// It implements io.Writer so it can be used with log.SetOutput.
type Buffer struct {
	mu      sync.RWMutex
	entries []Entry
	pos     int  // next write position
	full    bool // whether we've wrapped around
}

// New creates a ring buffer with the default capacity (1000).
func New() *Buffer {
	return &Buffer{entries: make([]Entry, defaultCapacity)}
}

// Write implements io.Writer. Each call stores one Entry.
// Multi-line writes are split; trailing newlines are stripped.
func (b *Buffer) Write(p []byte) (int, error) {
	now := time.Now()
	lines := bytes.Split(bytes.TrimRight(p, "\n"), []byte("\n"))

	b.mu.Lock()
	for _, line := range lines {
		msg := string(bytes.TrimRight(line, "\n"))
		if msg == "" {
			continue
		}
		b.entries[b.pos] = Entry{Time: now, Message: msg}
		b.pos++
		if b.pos >= len(b.entries) {
			b.pos = 0
			b.full = true
		}
	}
	b.mu.Unlock()
	return len(p), nil
}

// Last returns the most recent n entries, oldest first.
func (b *Buffer) Last(n int) []Entry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	total := b.pos
	if b.full {
		total = len(b.entries)
	}
	if n > total {
		n = total
	}
	if n == 0 {
		return nil
	}

	out := make([]Entry, n)
	if b.full {
		// Ring has wrapped — read from (pos-n) mod cap
		start := (b.pos - n + len(b.entries)) % len(b.entries)
		for i := 0; i < n; i++ {
			out[i] = b.entries[(start+i)%len(b.entries)]
		}
	} else {
		copy(out, b.entries[b.pos-n:b.pos])
	}
	return out
}
