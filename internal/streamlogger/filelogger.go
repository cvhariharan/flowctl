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

	"github.com/nxadm/tail"
)

const FileSyncInterval = 100 * time.Millisecond

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
func (f *FileLogManager) StreamLogs(ctx context.Context, execID string) (<-chan string, error) {
	logCh := make(chan string, 100)

	f.loggerMut.RLock()
	logger, exists := f.loggers[execID]
	f.loggerMut.RUnlock()

	go func() {
		defer close(logCh)

		select {
		case <-ctx.Done():
			log.Printf("stream logs for exec %s: error %v", execID, ctx.Err())
		default:
			var err error
			if exists {
				if fl, ok := logger.(*FileLogger); ok && !fl.IsClosed() {
					err = f.streamRealtimeLogs(ctx, execID, fl, logCh)
				} else {
					err = f.streamAllLogs(ctx, execID, logCh)
				}
			} else {
				err = f.streamAllLogs(ctx, execID, logCh)
			}

			if err != nil {
				log.Println(err)
			}
		}
	}()

	return logCh, nil
}

// streamAllLogs streams log lines from all log files for the given exec ID.
// This is used for executions that are not currently running.
func (f *FileLogManager) streamAllLogs(ctx context.Context, execID string, logCh chan<- string) error {
	entries, err := os.ReadDir(f.cfg.LogDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	var logFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if strings.HasPrefix(filename, execID+".") {
			logFiles = append(logFiles, filename)
		}
	}

	if len(logFiles) == 0 {
		return nil
	}

	// Sort files by index
	sort.Slice(logFiles, func(i, j int) bool {
		indexI := extractFileIndex(logFiles[i])
		indexJ := extractFileIndex(logFiles[j])
		return indexI < indexJ
	})

	// Stream from each file in order
	for _, filename := range logFiles {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			filePath := filepath.Join(f.cfg.LogDir, filename)
			if err := f.streamFromFile(ctx, filePath, logCh); err != nil {
				return fmt.Errorf("failed to stream from file %s: %w", filename, err)
			}
		}
	}

	return nil
}

// streamRealtimeLogs streams all archived logs plus active logs from the current file
// This is used for currently running executions.
func (f *FileLogManager) streamRealtimeLogs(ctx context.Context, execID string, fl *FileLogger, logCh chan<- string) error {
	// First stream all archived logs
	nextIndex := fl.nextFileIndex.Load()
	for i := int32(0); i < nextIndex-1; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			filename := fmt.Sprintf("%s.%d", execID, i)
			filePath := filepath.Join(f.cfg.LogDir, filename)

			if _, err := os.Stat(filePath); err == nil {
				if err := f.streamFromFile(ctx, filePath, logCh); err != nil {
					return fmt.Errorf("failed to stream from archived file %s: %w", filename, err)
				}
			}
		}
	}

	activeFilename := fmt.Sprintf("%s.%d", execID, nextIndex-1)
	activeFilePath := filepath.Join(f.cfg.LogDir, activeFilename)

	return f.followActiveFile(ctx, activeFilePath, fl.syncCh, logCh)
}

// streamFromFile reads all lines from a file and sends them to the channel
func (f *FileLogManager) streamFromFile(ctx context.Context, filePath string, logCh chan<- string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case logCh <- scanner.Text():
		}
	}

	return scanner.Err()
}

// followActiveFile reads from a file and follows it like tail -f, stopping when syncCh is closed
func (f *FileLogManager) followActiveFile(ctx context.Context, filePath string, syncCh <-chan struct{}, logCh chan<- string) error {
	tailConfig := tail.Config{
		Follow:    true,
		ReOpen:    true,
		MustExist: false,
		Location:  &tail.SeekInfo{Offset: 0, Whence: 0}, // Start from beginning
	}

	t, err := tail.TailFile(filePath, tailConfig)
	if err != nil {
		return fmt.Errorf("failed to tail file %s: %w", filePath, err)
	}
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-syncCh:
			// logger is closed, drain remaining lines
			for line := range t.Lines {
				logCh <- line.Text
			}
			return nil
		case line := <-t.Lines:
			logCh <- line.Text
		}
	}
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
	// ActionID is used to track the current action, this is a global value and cannot be used concurrently
	ActionID string
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

// SetActionID sets the action ID, this is a global value and should not be used concurrently
func (fl *FileLogger) SetActionID(id string) {
	fl.ActionID = id
}

func (fl *FileLogger) Write(p []byte) (int, error) {
	if err := fl.Checkpoint(fl.ActionID, "", p, LogMessageType); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Checkpoint can be used to set checkpoints for an action on a node like resuls, logs, errors etc.
func (fl *FileLogger) Checkpoint(id string, nodeID string, val interface{}, mtype MessageType) error {
	var sm StreamMessage
	sm.ActionID = fl.ActionID
	if id != "" {
		sm.ActionID = id
	}
	sm.NodeID = nodeID
	sm.Timestamp = time.Now().Format(time.RFC3339)
	switch mtype {
	case ErrMessageType:
		e, ok := val.(string)
		if !ok {
			return fmt.Errorf("expected string type for error got %T in stream checkpoint", val)
		}
		sm.MType = ErrMessageType
		sm.Val = []byte(e)
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
		sm.Val = data
	case LogMessageType:
		sm.MType = LogMessageType
		d, ok := val.([]byte)
		if !ok {
			return fmt.Errorf("expected []byte type for log got %T in stream checkpoint", val)
		}
		sm.MType = LogMessageType
		sm.Val = d
	case CancelledMessageType:
		e, ok := val.(string)
		if !ok {
			return fmt.Errorf("expected string type for cancelled got %T in stream checkpoint", val)
		}
		sm.MType = CancelledMessageType
		sm.Val = []byte(e)
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
