package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event holds all fields captured per CLI invocation.
type Event struct {
	Cmd        string
	Args       []string
	Flags      []string
	DurationMs int64
	Ok         bool
	Error      string
	Version    string
}

type wireEvent struct {
	Ts         string   `json:"ts"`
	Cmd        string   `json:"cmd"`
	Args       []string `json:"args"`
	Flags      []string `json:"flags"`
	DurationMs int64    `json:"duration_ms"`
	Ok         bool     `json:"ok"`
	Error      any      `json:"error"`
	Version    string   `json:"version"`
}

// Writer appends JSONL telemetry events with size-based rotation.
type Writer struct {
	mu       sync.Mutex
	dir      string
	maxBytes int64
	maxFiles int
	file     *os.File
}

// NewWriter opens (or creates) the telemetry file in dir.
func NewWriter(dir string, maxBytes int64, maxFiles int) (*Writer, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("telemetry: mkdir %s: %w", dir, err)
	}
	w := &Writer{dir: dir, maxBytes: maxBytes, maxFiles: maxFiles}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Writer) path() string {
	return filepath.Join(w.dir, "telemetry.jsonl")
}

func (w *Writer) open() error {
	f, err := os.OpenFile(w.path(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("telemetry: open: %w", err)
	}
	w.file = f
	return nil
}

func (w *Writer) rotate() error {
	_ = w.file.Close()

	// delete oldest retained file (.maxFiles-1), then shift .N → .N+1
	_ = os.Remove(fmt.Sprintf("%s.%d", w.path(), w.maxFiles-1))
	for i := w.maxFiles - 2; i >= 1; i-- {
		_ = os.Rename(
			fmt.Sprintf("%s.%d", w.path(), i),
			fmt.Sprintf("%s.%d", w.path(), i+1),
		)
	}
	_ = os.Rename(w.path(), w.path()+".1")

	return w.open()
}

// Record appends one event, rotating the file if needed.
func (w *Writer) Record(e Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var errField any
	if e.Error != "" {
		errField = e.Error
	}

	we := wireEvent{
		Ts:         time.Now().UTC().Format(time.RFC3339),
		Cmd:        e.Cmd,
		Args:       e.Args,
		Flags:      e.Flags,
		DurationMs: e.DurationMs,
		Ok:         e.Ok,
		Error:      errField,
		Version:    e.Version,
	}

	line, err := json.Marshal(we)
	if err != nil {
		return fmt.Errorf("telemetry: marshal: %w", err)
	}
	line = append(line, '\n')

	info, err := w.file.Stat()
	if err == nil && info.Size()+int64(len(line)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return err
		}
	}

	_, err = w.file.Write(line)
	return err
}

// Close flushes and closes the underlying file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}
