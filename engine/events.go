/* MAP
 * §1 package & imports   L8-L16
 * §2 Event payloads      L18-L45
 * §3 Emitter methods     L47-L114
 */

// §1 package & imports
package main

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// §2 Event payloads
type EngineEvent struct {
	Type        string  `json:"type"`
	DoneFiles   int64   `json:"doneFiles,omitempty"`
	TotalFiles  int64   `json:"totalFiles,omitempty"`
	DoneBytes   int64   `json:"doneBytes,omitempty"`
	TotalBytes  int64   `json:"totalBytes,omitempty"`
	CurrentFile string  `json:"currentFile,omitempty"`
	SpeedBps    float64 `json:"speedBps,omitempty"`
	OutputPath  string  `json:"outputPath,omitempty"`
	InputBytes  int64   `json:"inputBytes,omitempty"`
	OutputBytes int64   `json:"outputBytes,omitempty"`
	Files       int64   `json:"files,omitempty"`
	ElapsedMs   int64   `json:"elapsedMs,omitempty"`
	Skipped     bool    `json:"skipped,omitempty"`
	Message     string  `json:"message,omitempty"`
}

type Emitter struct {
	mu       sync.Mutex
	start    time.Time
	lastEmit time.Time
}

// §3 Emitter methods
func NewEmitter() *Emitter {
	now := time.Now()
	return &Emitter{start: now, lastEmit: time.Time{}}
}

func (e *Emitter) emit(ev EngineEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()
	line, err := json.Marshal(ev)
	if err != nil {
		return
	}
	_, _ = os.Stdout.Write(append(line, '\n'))
}

func (e *Emitter) EmitScan(totalFiles, totalBytes int64) {
	e.emit(EngineEvent{
		Type:       "scan",
		TotalFiles: totalFiles,
		TotalBytes: totalBytes,
	})
}

func (e *Emitter) EmitProgress(doneFiles, totalFiles, doneBytes, totalBytes int64, currentFile string, force bool) {
	now := time.Now()
	e.mu.Lock()
	if !force && !e.lastEmit.IsZero() && now.Sub(e.lastEmit) < ProgressIntervalMs*time.Millisecond {
		e.mu.Unlock()
		return
	}
	e.lastEmit = now
	elapsedSec := now.Sub(e.start).Seconds()
	e.mu.Unlock()

	var speed float64
	if elapsedSec > 0.05 && doneBytes > 0 {
		speed = float64(doneBytes) / elapsedSec
	}
	e.emit(EngineEvent{
		Type:        "progress",
		DoneFiles:   doneFiles,
		TotalFiles:  totalFiles,
		DoneBytes:   doneBytes,
		TotalBytes:  totalBytes,
		CurrentFile: currentFile,
		SpeedBps:    speed,
	})
}

func (e *Emitter) EmitDone(outputPath string, inputBytes, outputBytes, files int64, skipped bool) {
	elapsedMs := time.Since(e.start).Milliseconds()
	e.emit(EngineEvent{
		Type:        "done",
		OutputPath:  outputPath,
		InputBytes:  inputBytes,
		OutputBytes: outputBytes,
		Files:       files,
		ElapsedMs:   elapsedMs,
		Skipped:     skipped,
	})
}

func (e *Emitter) EmitError(msg string) {
	e.emit(EngineEvent{
		Type:    "error",
		Message: msg,
	})
}
