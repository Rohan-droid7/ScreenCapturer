package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger provides capped and rotating log file output.
type Logger struct {
	mu           sync.Mutex
	logFilePath  string
	maxSizeBytes int64
	maxBackups   int
	file         *os.File
	stdoutAlso   bool
}

var globalLogger *Logger
var once sync.Once

// InitGlobal initializes the global logger.
func InitGlobal(logDir string, maxMB int, maxBackups int, stdoutAlso bool) (*Logger, error) {
	if maxMB <= 0 {
		maxMB = 5
	}
	if maxBackups <= 0 {
		maxBackups = 3
	}
	if logDir == "" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = "."
		}
		logDir = filepath.Join(appData, "ScreenCapturer")
	}

	if err := os.MkdirAll(logDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create log dir: %w", err)
	}

	logPath := filepath.Join(logDir, "screencap.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	l := &Logger{
		logFilePath:  logPath,
		maxSizeBytes: int64(maxMB) * 1024 * 1024,
		maxBackups:   maxBackups,
		file:         f,
		stdoutAlso:   stdoutAlso,
	}

	once.Do(func() {
		globalLogger = l
	})

	return l, nil
}

// Get returns the global logger or a stdout fallback.
func Get() *Logger {
	if globalLogger == nil {
		return &Logger{stdoutAlso: true}
	}
	return globalLogger
}

// Close closes the underlying log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

func (l *Logger) writeEntry(level, msg string) {
	now := time.Now().Format("2006-01-02 15:04:05.000")
	line := fmt.Sprintf("[%s] [%s] %s\n", now, level, msg)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.stdoutAlso {
		// Only write if handle is valid
		if os.Stdout != nil {
			_, _ = os.Stdout.WriteString(line)
		}
	}

	if l.file == nil {
		return
	}

	fi, err := l.file.Stat()
	if err == nil && fi.Size()+int64(len(line)) > l.maxSizeBytes {
		l.rotateLocked()
	}

	_, _ = l.file.WriteString(line)
	_ = l.file.Sync()
}

func (l *Logger) rotateLocked() {
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}

	for i := l.maxBackups - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", l.logFilePath, i)
		dst := fmt.Sprintf("%s.%d", l.logFilePath, i+1)
		_ = os.Rename(src, dst)
	}

	_ = os.Rename(l.logFilePath, fmt.Sprintf("%s.1", l.logFilePath))

	f, err := os.OpenFile(l.logFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err == nil {
		l.file = f
	}
}

// Infof formats and logs an info message.
func (l *Logger) Infof(format string, args ...any) {
	l.writeEntry("INFO", fmt.Sprintf(format, args...))
}

// Warnf formats and logs a warning message.
func (l *Logger) Warnf(format string, args ...any) {
	l.writeEntry("WARN", fmt.Sprintf(format, args...))
}

// Errorf formats and logs an error message.
func (l *Logger) Errorf(format string, args ...any) {
	l.writeEntry("ERROR", fmt.Sprintf(format, args...))
}

// LogCaptureTiming logs timings for screen capture and image encoding (NO image data, NO tokens).
func (l *Logger) LogCaptureTiming(clientIP, engine, format string, captureDur, encodeDur, totalDur time.Duration, sizeBytes int) {
	l.writeEntry("TIMING", fmt.Sprintf(
		"client=%s engine=%s format=%s capture_ms=%.2f encode_ms=%.2f total_ms=%.2f size_bytes=%d",
		clientIP, engine, format,
		float64(captureDur.Microseconds())/1000.0,
		float64(encodeDur.Microseconds())/1000.0,
		float64(totalDur.Microseconds())/1000.0,
		sizeBytes,
	))
}
