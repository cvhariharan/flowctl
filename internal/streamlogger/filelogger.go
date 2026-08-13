package streamlogger

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	FileSyncInterval = 100 * time.Millisecond
	LogPollInterval  = 100 * time.Millisecond
)

// extractFileIndex extracts the numeric index from a log filename
func extractFileIndex(filename string) int {
	lastDot := strings.LastIndex(filename, ".")
	if lastDot == -1 {
		return 0
	}

	indexStr := filename[lastDot+1:]
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return 0
	}

	return index
}

type FileLogManagerCfg struct {
	// RetentionTime is used to determine files that are old enough to be deleted.
	// If the file modification is older than RetentionTime, it will be deleted.
	RetentionTime time.Duration

	// MaxSizeBytes is used by the FileLogger to rotate files.
	// If the written bytes exceed this value, a new file will be created
	MaxSizeBytes int64

	// ScanInterval is the interval with which the FileLogManager scans the log directory to determine
	// files that should be deleted
	ScanInterval time.Duration

	// LogDir stores the log files created by the FileLogger
	LogDir string
}

type FileLogManager struct {
	cfg FileLogManagerCfg
	// loggers is used to track active loggers, this is used for file deletion checks
	loggers map[string]Logger
	// loggerMut is used in conjunction with loggers map
	loggerMut sync.RWMutex
	// scanTicker uses the ScanInterval from the cfg and is used to run periodic scans
	scanTicker *time.Ticker
}

// NewFileLogManager creates a log manager that uses files as the storage backend.
// FileLogManager supports retention time to clean up old log files
func NewFileLogManager(cfg FileLogManagerCfg) LogManager {
	if cfg.ScanInterval == 0 {
		cfg.ScanInterval = 1 * time.Hour
	}

	if cfg.LogDir == "" {
		cfg.LogDir = os.TempDir()
	}

	return &FileLogManager{
		cfg:        cfg,
		loggers:    make(map[string]Logger),
		scanTicker: time.NewTicker(cfg.ScanInterval),
	}
}

// NewLogger creates a new FileLogger.
// This is used by the task handler to create a new logger for each flow execution.
func (f *FileLogManager) NewLogger(id string) (Logger, error) {
	fl, err := newFileLogger(id, f.cfg.LogDir, FileSyncInterval, f.cfg.MaxSizeBytes)
	if err != nil {
		return nil, err
	}

	f.loggerMut.Lock()
	defer f.loggerMut.Unlock()

	f.loggers[id] = fl
	return fl, nil
}

// LoggerExists checks if an active logger exists for the given exec ID
func (f *FileLogManager) LoggerExists(execID string) bool {
	f.loggerMut.RLock()
	logger, exists := f.loggers[execID]
	f.loggerMut.RUnlock()

	if !exists {
		return false
	}

	if fl, ok := logger.(*FileLogger); ok {
		isClosed := fl.IsClosed()
		return !isClosed
	}

	return true
}

// StreamLogs creates and returns a channel that streams log lines for the given exec ID
// It filters logs to show only the highest retry attempt for each action
func (f *FileLogManager) StreamLogs(ctx context.Context, execID string, actionRetries map[string]int32) (<-chan string, error) {
	logCh := make(chan string, 100)

	f.loggerMut.RLock()
	logger, exists := f.loggers[execID]
	f.loggerMut.RUnlock()

	closed := make(chan struct{})
	if fl, ok := logger.(*FileLogger); exists && ok {
		closed = fl.syncCh
	} else {
		close(closed)
	}

	go func() {
		defer close(logCh)

		if err := f.followLogs(ctx, execID, closed, actionRetries, logCh); err != nil && ctx.Err() == nil {
			log.Printf("stream logs for exec %s: %v", execID, err)
		}
	}()

	return logCh, nil
}

// getLogFiles returns the sorted list of log file names for the given execID.
func (f *FileLogManager) getLogFiles(execID string) ([]string, error) {
	entries, err := os.ReadDir(f.cfg.LogDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read log directory: %w", err)
	}

	prefix := execID + "."
	var logFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			logFiles = append(logFiles, entry.Name())
		}
	}

	sort.Slice(logFiles, func(i, j int) bool {
		return extractFileIndex(logFiles[i]) < extractFileIndex(logFiles[j])
	})

	return logFiles, nil
}

// readCompleteLines reads from offset and returns the number of bytes belonging to complete
// newline-terminated records. A trailing partial record is deliberately left unread so the next
// poll can retry it after the writer flushes the remainder.
func (f *FileLogManager) readCompleteLines(ctx context.Context, filePath string, offset int64, actionRetries map[string]int32, logCh chan<- string) (int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return 0, err
	}

	reader := bufio.NewReader(file)
	var consumed int64
	for {
		line, readErr := reader.ReadString('\n')
		if strings.HasSuffix(line, "\n") {
			consumed += int64(len(line))
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")
			if f.shouldStreamLogLine(line, actionRetries) {
				select {
				case logCh <- line:
				case <-ctx.Done():
					return consumed, ctx.Err()
				}
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				return consumed, nil
			}
			return consumed, readErr
		}
	}
}

// followLogs polls file offsets and follows segment rotation until the logger closes. The same
// path is used for active and completed executions; a completed execution supplies an already
// closed channel and is replayed from segment zero.
func (f *FileLogManager) followLogs(ctx context.Context, execID string, closed <-chan struct{}, actionRetries map[string]int32, logCh chan<- string) error {
	index := 0
	var offset int64
	finalPass := false
	ticker := time.NewTicker(LogPollInterval)
	defer ticker.Stop()

	for {
		filePath := filepath.Join(f.cfg.LogDir, fmt.Sprintf("%s.%d", execID, index))
		consumed, err := f.readCompleteLines(ctx, filePath, offset, actionRetries, logCh)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to read log segment %d: %w", index, err)
		}
		offset += consumed

		nextPath := filepath.Join(f.cfg.LogDir, fmt.Sprintf("%s.%d", execID, index+1))
		if _, err := os.Stat(nextPath); err == nil {
			index++
			offset = 0
			finalPass = false
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("failed to inspect next log segment: %w", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-closed:
			// Closing flushes before signalling. Repeat once so a rotation that raced the
			// previous stat and every last buffered record are observed.
			if finalPass {
				return nil
			}
			finalPass = true
			continue
		default:
			finalPass = false
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-closed:
			finalPass = true
		case <-ticker.C:
		}
	}
}

// shouldStreamLogLine checks if a log line should be streamed based on retry filtering
func (f *FileLogManager) shouldStreamLogLine(line string, actionRetries map[string]int32) bool {
	var msg StreamMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		// If we can't parse, stream the line anyway (backward compatibility)
		return true
	}

	// Backwards compatibility: if retry field is 0 (not present in old logs), treat as 1
	logRetry := msg.Retry
	if logRetry == 0 {
		logRetry = 1
	}

	// Show only logs from the highest retry attempt for each action
	maxRetry, exists := actionRetries[msg.ActionID]
	if !exists {
		maxRetry = 1 // Default to 1 since we always increment before execution
	}

	return logRetry == maxRetry
}

// GetRawLogs writes all raw log file segments for the given execID to w, in order.
// Returns an error if the logger for this execID is still active (execution still running).
func (f *FileLogManager) GetRawLogs(ctx context.Context, execID string, w io.Writer) error {
	if f.LoggerExists(execID) {
		return fmt.Errorf("execution %s is still running", execID)
	}

	logFiles, err := f.getLogFiles(execID)
	if err != nil {
		return err
	}

	if len(logFiles) == 0 {
		return nil
	}

	for _, filename := range logFiles {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			file, err := os.Open(filepath.Join(f.cfg.LogDir, filename))
			if err != nil {
				return fmt.Errorf("failed to open log file %s: %w", filename, err)
			}
			_, copyErr := io.Copy(w, file)
			file.Close()
			if copyErr != nil {
				return fmt.Errorf("failed to read log file %s: %w", filename, copyErr)
			}
		}
	}

	return nil
}

// Run starts the scan loop.
// This is a blocking call and should be run from a goroutine.
func (f *FileLogManager) Run(ctx context.Context, l *slog.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-f.scanTicker.C:
			if err := f.run(ctx, l); err != nil {
				l.Error("failed to run retention scan", "error", err)
			}
		}
	}
}

// run performs the retention scan and deletes old files
func (f *FileLogManager) run(ctx context.Context, l *slog.Logger) error {
	if f.cfg.RetentionTime <= 0 {
		return nil
	}

	entries, err := os.ReadDir(f.cfg.LogDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	now := time.Now()
	var filesToDelete []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			l.Warn("failed to get file info", "file", entry.Name(), "error", err)
			continue
		}

		// Check if file is older than retention time
		if now.Sub(info.ModTime()) > f.cfg.RetentionTime {
			// Check if file belongs to an active (not closed) logger
			if !f.isFileInUse(entry.Name()) {
				filesToDelete = append(filesToDelete, filepath.Join(f.cfg.LogDir, entry.Name()))
			}
		}
	}

	// Delete files in a goroutine to avoid blocking
	if len(filesToDelete) > 0 {
		go f.deleteFiles(ctx, filesToDelete, l)
	}

	return nil
}

// isFileInUse checks if a file belongs to an active (not closed) logger
func (f *FileLogManager) isFileInUse(filename string) bool {
	f.loggerMut.RLock()
	defer f.loggerMut.RUnlock()

	lastDot := strings.LastIndex(filename, ".")
	if lastDot == -1 {
		return false
	}
	execID := filename[:lastDot]

	logger, exists := f.loggers[execID]
	if !exists {
		return false
	}

	if fl, ok := logger.(*FileLogger); ok {
		return !fl.IsClosed()
	}
	return true
}

// deleteFiles deletes the given files in the background
func (f *FileLogManager) deleteFiles(ctx context.Context, files []string, l *slog.Logger) {
	for _, file := range files {
		select {
		case <-ctx.Done():
			return
		default:
			if err := os.Remove(file); err != nil {
				l.Warn("failed to delete old log file", "file", file, "error", err)
			} else {
				l.Debug("deleted old log file", "file", file)
			}
		}
	}
}

// FileLogger implements io.Writer and is meant to be used for a single execution
type FileLogger struct {
	// ExecID is the execution ID of the associated flow
	ExecID string
	// buffer stores the messages from executions
	buffer    *bytes.Buffer
	bufferMut sync.RWMutex
	// logDirPath is the directory where all log files will be stored
	logDirPath string
	// flushTicker is used to periodically write values from buffer to file
	flushTicker *time.Ticker
	// syncCh is used to track if the logger is closed
	syncCh  chan struct{}
	runOnce sync.Once
	// writtenCount is used to track the written bytes by this logger
	writtenCount atomic.Int64
	// maxSize is the max file size in bytes before it is rotated
	maxSize int64
	// nextFileIndex is the file index for the next file after rotation
	nextFileIndex atomic.Int32
	// currentFile is the current open log file
	currentFile atomic.Pointer[os.File]
}

func newFileLogger(execID string, logDirPath string, syncInterval time.Duration, maxSize int64) (Logger, error) {
	fl := &FileLogger{
		ExecID:      execID,
		logDirPath:  logDirPath,
		flushTicker: time.NewTicker(syncInterval),
		syncCh:      make(chan struct{}),
		buffer:      new(bytes.Buffer),
		maxSize:     maxSize,
	}

	if err := fl.rotateFile(); err != nil {
		return nil, err
	}

	go fl.sync()

	return fl, nil
}

func (fl *FileLogger) IsClosed() bool {
	select {
	case <-fl.syncCh:
		return true
	default:
		return false
	}
}

// rotateFile creates a new file with the next file index and swaps the current file pointer
func (fl *FileLogger) rotateFile() error {
	f, err := os.OpenFile(filepath.Join(fl.logDirPath, fmt.Sprintf("%s.%d", fl.ExecID, fl.nextFileIndex.Load())), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("could not create log file for exec=%s and index=%d: %w", fl.ExecID, fl.nextFileIndex.Load(), err)
	}
	fl.nextFileIndex.Add(1)
	if old := fl.currentFile.Swap(f); old != nil {
		old.Close()
	}
	return nil
}

// Close flushes the buffer and closes the logger and file
func (fl *FileLogger) Close() error {
	fl.runOnce.Do(func() {
		fl.flushTicker.Stop()
		// Flush any pending data to file
		fl.filesync()
		close(fl.syncCh)
	})
	f := fl.currentFile.Load()
	return f.Close()
}

// GetID returns the exec ID
func (fl *FileLogger) GetID() string {
	return fl.ExecID
}

func (fl *FileLogger) Write(p []byte) (int, error) {
	if err := fl.Checkpoint("", "", p, LogMessageType, 0); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Checkpoint can be used to set checkpoints for an action on a node like resuls, logs, errors etc.
func (fl *FileLogger) Checkpoint(id string, nodeID string, val interface{}, mtype MessageType, retry int32) error {
	var sm StreamMessage
	sm.ActionID = id
	sm.NodeID = nodeID
	sm.Timestamp = time.Now().Format(time.RFC3339)
	sm.Retry = retry
	switch mtype {
	case ErrMessageType:
		e, ok := val.(string)
		if !ok {
			return fmt.Errorf("expected string type for error got %T in stream checkpoint", val)
		}
		sm.MType = ErrMessageType
		sm.Val = e
	case ResultMessageType:
		r, ok := val.(map[string]string)
		if !ok {
			return fmt.Errorf("expected map[string]string type got %T in stream checkpoint", val)
		}
		data, err := json.Marshal(r)
		if err != nil {
			return fmt.Errorf("could not marshal result for result message type in stream message %s: %w", id, err)
		}
		sm.MType = ResultMessageType
		sm.Val = string(data)
	case LogMessageType:
		sm.MType = LogMessageType
		d, ok := val.([]byte)
		if !ok {
			return fmt.Errorf("expected []byte type for log got %T in stream checkpoint", val)
		}
		sm.MType = LogMessageType
		sm.Val = string(d)
	case CancelledMessageType:
		e, ok := val.(string)
		if !ok {
			return fmt.Errorf("expected string type for cancelled got %T in stream checkpoint", val)
		}
		sm.MType = CancelledMessageType
		sm.Val = e
	}

	msgBytes, err := json.Marshal(sm)
	if err != nil {
		return fmt.Errorf("could not marshal stream message: %w", err)
	}

	if fl.IsClosed() {
		return fmt.Errorf("logger has been closed")
	}
	fl.bufferMut.Lock()
	defer fl.bufferMut.Unlock()
	_, err = fl.buffer.Write(msgBytes)
	if err != nil {
		return err
	}
	_, err = fl.buffer.Write([]byte("\n"))
	return err
}

// sync uses the flushticker to sync buffer with file
func (fl *FileLogger) sync() error {
	for {
		select {
		case <-fl.syncCh:
			return nil
		case <-fl.flushTicker.C:
			fl.filesync()
		}
	}
}

// filesync copies the contents from buffer to the current logger file
func (fl *FileLogger) filesync() error {
	// Check if rotation is needed before acquiring any locks
	if fl.maxSize > 0 && fl.writtenCount.Load() > fl.maxSize {
		if err := fl.rotateFile(); err != nil {
			return err
		}
		fl.writtenCount.Store(0)
	}

	// Now copy buffer contents to file
	fl.bufferMut.Lock()
	n, err := io.Copy(fl.currentFile.Load(), fl.buffer)
	fl.writtenCount.Add(n)
	fl.buffer.Reset()
	if err != nil {
		fl.bufferMut.Unlock()
		return err
	}
	fl.bufferMut.Unlock()

	return fl.currentFile.Load().Sync()
}
