// Package system provides pure domain entities and invariants for system maintenance, logging, and metrics.
// Pure domain: zero external dependencies outside standard library.
package system

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrEmptyLogLevel   = errors.New("log level cannot be empty")
	ErrEmptyLogMessage = errors.New("log message cannot be empty")
	ErrNegativeMetric  = errors.New("metric counter cannot be negative")
)

type LogLevel string

const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
)

// LogEntry represents an immutable structured log record.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     LogLevel  `json:"level"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
}

// NewLogEntry creates and validates a system LogEntry.
func NewLogEntry(level LogLevel, msg, source string, now time.Time) (*LogEntry, error) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return nil, ErrEmptyLogMessage
	}
	level = LogLevel(strings.ToUpper(strings.TrimSpace(string(level))))
	if level == "" {
		return nil, ErrEmptyLogLevel
	}
	return &LogEntry{
		Timestamp: now,
		Level:     level,
		Message:   msg,
		Source:    strings.TrimSpace(source),
	}, nil
}

// MetricsSnapshot captures runtime resource consumption.
type MetricsSnapshot struct {
	AllocBytes   uint64    `json:"allocBytes"`
	TotalAlloc   uint64    `json:"totalAlloc"`
	SysBytes     uint64    `json:"sysBytes"`
	NumGC        uint32    `json:"numGc"`
	Goroutines   int       `json:"goroutines"`
	CollectedAt  time.Time `json:"collectedAt"`
}

// VersionInfo represents the semantic versioning state of the application.
type VersionInfo struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	UpdateAvailable bool  `json:"updateAvailable"`
	Channel        string `json:"channel"`
}

// CheckUpdate evaluates whether a target version exceeds the current deployed version.
func CheckUpdate(current, latest, channel string) VersionInfo {
	current = strings.TrimSpace(current)
	latest = strings.TrimSpace(latest)
	return VersionInfo{
		CurrentVersion: current,
		LatestVersion:  latest,
		UpdateAvailable: latest != "" && current != "" && latest != current,
		Channel:        channel,
	}
}
