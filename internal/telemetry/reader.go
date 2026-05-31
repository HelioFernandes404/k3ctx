package telemetry

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// CommandStat holds aggregated telemetry metrics for one command.
type CommandStat struct {
	Cmd         string  `json:"cmd"`
	Count       int     `json:"count"`
	OKCount     int     `json:"ok_count"`
	ErrorCount  int     `json:"error_count"`
	AvgDuration float64 `json:"avg_duration_ms"`
	ErrorRate   float64 `json:"error_rate"`
}

// ReadLastN returns up to n events from the telemetry directory in newest-first order.
// Files are read in order: telemetry.jsonl, telemetry.jsonl.1, telemetry.jsonl.2.
// Blank lines and invalid JSON are silently skipped.
func ReadLastN(dir string, n int) ([]map[string]any, error) {
	if n <= 0 {
		return []map[string]any{}, nil
	}

	files := []string{
		filepath.Join(dir, "telemetry.jsonl"),
		filepath.Join(dir, "telemetry.jsonl.1"),
		filepath.Join(dir, "telemetry.jsonl.2"),
	}

	var collected []map[string]any
	for _, path := range files {
		if len(collected) >= n {
			break
		}
		lines, err := readValidLines(path)
		if err != nil {
			continue
		}
		// Take the tail of this file (newest lines), appended in reverse (newest-first).
		need := n - len(collected)
		start := len(lines) - need
		if start < 0 {
			start = 0
		}
		for i := len(lines) - 1; i >= start; i-- {
			collected = append(collected, lines[i])
		}
	}
	return collected, nil
}

// Aggregate reads all available telemetry events and returns per-command statistics
// sorted by count descending.
func Aggregate(dir string) ([]CommandStat, error) {
	files := []string{
		filepath.Join(dir, "telemetry.jsonl"),
		filepath.Join(dir, "telemetry.jsonl.1"),
		filepath.Join(dir, "telemetry.jsonl.2"),
	}

	type accumulator struct {
		count      int
		okCount    int
		totalDurMs float64
	}
	acc := map[string]*accumulator{}

	for _, path := range files {
		lines, err := readValidLines(path)
		if err != nil {
			continue
		}
		for _, ev := range lines {
			cmd, _ := ev["cmd"].(string)
			if cmd == "" {
				continue
			}
			if _, ok := acc[cmd]; !ok {
				acc[cmd] = &accumulator{}
			}
			a := acc[cmd]
			a.count++
			if ok, _ := ev["ok"].(bool); ok {
				a.okCount++
			}
			if d, ok := ev["duration_ms"].(float64); ok {
				a.totalDurMs += d
			}
		}
	}

	stats := make([]CommandStat, 0, len(acc))
	for cmd, a := range acc {
		errorCount := a.count - a.okCount
		avgDur := 0.0
		if a.count > 0 {
			avgDur = a.totalDurMs / float64(a.count)
		}
		errorRate := 0.0
		if a.count > 0 {
			errorRate = float64(errorCount) / float64(a.count)
		}
		stats = append(stats, CommandStat{
			Cmd:         cmd,
			Count:       a.count,
			OKCount:     a.okCount,
			ErrorCount:  errorCount,
			AvgDuration: avgDur,
			ErrorRate:   errorRate,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Count > stats[j].Count
	})
	return stats, nil
}

func readValidLines(path string) ([]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, err
	}
	defer f.Close()

	var out []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		out = append(out, ev)
	}
	return out, sc.Err()
}
