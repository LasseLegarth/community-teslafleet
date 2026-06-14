// Package recorder appends every telemetry field update to a JSONL file so value
// changes can be debugged over time (e.g. reviewing a whole drive afterwards).
// Each line: {"t":"<RFC3339>","vin":"...","field":"Soc","value":58.3}.
package recorder

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"time"
)

type Recorder struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	f        *os.File
	w        *bufio.Writer
	size     int64
	log      *slog.Logger
}

type line struct {
	T     string `json:"t"`
	VIN   string `json:"vin"`
	Field string `json:"field"`
	Value any    `json:"value"`
}

// New opens (appends to) the JSONL file. maxMB rotates the file to <path>.1 when
// exceeded (one backup kept). A path of "" disables recording (returns nil, nil).
func New(path string, maxMB int, log *slog.Logger) (*Recorder, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	info, _ := f.Stat()
	var size int64
	if info != nil {
		size = info.Size()
	}
	if maxBytes := int64(maxMB) * 1024 * 1024; maxBytes <= 0 {
		maxMB = 100
	}
	r := &Recorder{
		path:     path,
		maxBytes: int64(maxMB) * 1024 * 1024,
		f:        f,
		w:        bufio.NewWriter(f),
		size:     size,
		log:      log,
	}
	// Flush periodically so a crash/drive-review doesn't lose the buffer tail.
	go r.flushLoop()
	log.Info("telemetry recording enabled", "path", path, "max_mb", maxMB)
	return r, nil
}

// Record appends one field update. Safe to call with a nil *Recorder (no-op).
func (r *Recorder) Record(vin, field string, value any) {
	if r == nil {
		return
	}
	b, err := json.Marshal(line{T: time.Now().UTC().Format(time.RFC3339Nano), VIN: vin, Field: field, Value: value})
	if err != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	n, _ := r.w.Write(b)
	r.w.WriteByte('\n')
	r.size += int64(n) + 1
	if r.size >= r.maxBytes {
		r.rotate()
	}
}

func (r *Recorder) rotate() {
	r.w.Flush()
	r.f.Close()
	_ = os.Rename(r.path, r.path+".1") // keep one backup
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		r.log.Warn("recorder rotate reopen failed", "err", err)
		return
	}
	r.f = f
	r.w = bufio.NewWriter(f)
	r.size = 0
}

func (r *Recorder) flushLoop() {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for range t.C {
		r.mu.Lock()
		r.w.Flush()
		r.mu.Unlock()
	}
}

func (r *Recorder) Close() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.w.Flush()
	r.f.Close()
}
