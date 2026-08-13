package streamlogger

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileLogger_BasicOperations(t *testing.T) {
	tmpDir := t.TempDir()
	execID := "test-exec-123"

	logger, err := newFileLogger(execID, tmpDir, 50*time.Millisecond, 0)
	if err != nil {
		t.Fatalf("newFileLogger() error = %v", err)
	}
	defer logger.Close()

	if got := logger.GetID(); got != execID {
		t.Errorf("GetID() = %s, want %s", got, execID)
	}

	actionID := "action-456"
	nodeLogger := NewNodeContextLogger(logger, actionID, "", 0)
	testData := "test log data\n"
	n, err := nodeLogger.Write([]byte(testData))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write() returned %d, want %d", n, len(testData))
	}

	time.Sleep(100 * time.Millisecond)
	filePath := filepath.Join(tmpDir, "test-exec-123.0")
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	// Parse the JSON stream message
	var sm StreamMessage
	lines := strings.Split(strings.TrimSpace(string(fileData)), "\n")
	if len(lines) != 1 {
		t.Fatalf("Expected 1 line in file, got %d", len(lines))
	}
	if err := json.Unmarshal([]byte(lines[0]), &sm); err != nil {
		t.Fatalf("Failed to unmarshal stream message: %v", err)
	}
	if sm.Val != testData {
		t.Errorf("stream message value = %q, want %q", sm.Val, testData)
	}
	if sm.MType != LogMessageType {
		t.Errorf("stream message type = %v, want %v", sm.MType, LogMessageType)
	}
	if sm.ActionID != actionID {
		t.Errorf("stream message ActionID = %q, want %q", sm.ActionID, actionID)
	}
}

func TestFileLogger_MultipleWrites(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := newFileLogger("exec-id", tmpDir, 50*time.Millisecond, 0)
	if err != nil {
		t.Fatalf("newFileLogger() error = %v", err)
	}
	defer logger.Close()

	chunks := []string{"first\n", "second\n", "third\n"}
	for _, chunk := range chunks {
		_, err := logger.Write([]byte(chunk))
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	time.Sleep(100 * time.Millisecond)
	filePath := filepath.Join(tmpDir, "exec-id.0")
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	// Parse each JSON stream message
	lines := strings.Split(strings.TrimSpace(string(fileData)), "\n")
	if len(lines) != len(chunks) {
		t.Fatalf("Expected %d lines in file, got %d", len(chunks), len(lines))
	}

	for i, line := range lines {
		var sm StreamMessage
		if err := json.Unmarshal([]byte(line), &sm); err != nil {
			t.Fatalf("Failed to unmarshal stream message %d: %v", i, err)
		}
		if sm.Val != chunks[i] {
			t.Errorf("stream message %d value = %q, want %q", i, sm.Val, chunks[i])
		}
		if sm.MType != LogMessageType {
			t.Errorf("stream message %d type = %v, want %v", i, sm.MType, LogMessageType)
		}
	}
}

func TestFileLogger_LargeData(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := newFileLogger("exec-id", tmpDir, 50*time.Millisecond, 0)
	if err != nil {
		t.Fatalf("newFileLogger() error = %v", err)
	}
	defer logger.Close()

	pattern := "This is a test log line with some content to make it reasonably sized.\n"
	repetitions := 1024 * 16
	largeData := strings.Repeat(pattern, repetitions)

	n, err := logger.Write([]byte(largeData))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != len(largeData) {
		t.Errorf("Write() returned %d, want %d", n, len(largeData))
	}

	time.Sleep(100 * time.Millisecond)
	filePath := filepath.Join(tmpDir, "exec-id.0")
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	// Parse the JSON stream message and verify the large data
	lines := strings.Split(strings.TrimSpace(string(fileData)), "\n")
	if len(lines) != 1 {
		t.Fatalf("Expected 1 line in file, got %d", len(lines))
	}
	var sm StreamMessage
	if err := json.Unmarshal([]byte(lines[0]), &sm); err != nil {
		t.Fatalf("Failed to unmarshal stream message: %v", err)
	}
	if len(sm.Val) != len(largeData) {
		t.Errorf("stream message value length = %d, want %d", len(sm.Val), len(largeData))
	}
	if sm.Val != largeData {
		t.Errorf("stream message value does not match expected large data")
	}
}

func TestFileLogger_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := newFileLogger("exec-id", tmpDir, 50*time.Millisecond, 0)
	if err != nil {
		t.Fatalf("newFileLogger() error = %v", err)
	}
	defer logger.Close()

	_, err = logger.Write([]byte("test"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	filePath := filepath.Join(tmpDir, "exec-id.0")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0644 {
		t.Errorf("file permissions = %v, want 0644", perm)
	}
}

func TestFileLogger_InvalidPath(t *testing.T) {
	invalidPath := "/invalid/path/that/does/not/exist/test.log"
	logger, err := newFileLogger("exec-id", invalidPath, 50*time.Millisecond, 0)
	if err == nil {
		t.Fatal("newFileLogger() with invalid path should return error")
	}
	if logger != nil {
		t.Fatal("newFileLogger() should return nil logger on error")
	}
}

func TestFileLogger_IsClosed(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := newFileLogger("exec-id", tmpDir, 50*time.Millisecond, 0)
	if err != nil {
		t.Fatalf("newFileLogger() error = %v", err)
	}

	fl := logger.(*FileLogger)

	if fl.IsClosed() {
		t.Error("IsClosed() = true, want false (logger just created)")
	}

	_, err = logger.Write([]byte("test data\n"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if fl.IsClosed() {
		t.Error("IsClosed() = true, want false (after write)")
	}

	logger.Close()

	if !fl.IsClosed() {
		t.Error("IsClosed() = false, want true (after Close)")
	}
	_, err = logger.Write([]byte("should fail"))
	if err == nil {
		t.Error("Write() after Close() should return error")
	}
}

func TestFileLogger_FileRotation(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := newFileLogger("exec-id", tmpDir, 50*time.Millisecond, 10)
	if err != nil {
		t.Fatalf("newFileLogger() error = %v", err)
	}
	defer logger.Close()

	data1 := "12345678901"
	_, err = logger.Write([]byte(data1))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	file0 := filepath.Join(tmpDir, "exec-id.0")
	fileData, err := os.ReadFile(file0)
	if err != nil {
		t.Fatalf("ReadFile() file0 error = %v", err)
	}

	// Parse the first file's JSON stream message
	lines := strings.Split(strings.TrimSpace(string(fileData)), "\n")
	if len(lines) != 1 {
		t.Fatalf("Expected 1 line in file0, got %d", len(lines))
	}
	var sm1 StreamMessage
	if err := json.Unmarshal([]byte(lines[0]), &sm1); err != nil {
		t.Fatalf("Failed to unmarshal stream message from file0: %v", err)
	}
	if sm1.Val != data1 {
		t.Errorf("file0 stream message value = %q, want %q", sm1.Val, data1)
	}

	data2 := "ABCDEFGHIJK"
	_, err = logger.Write([]byte(data2))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	file1 := filepath.Join(tmpDir, "exec-id.1")
	fileData, err = os.ReadFile(file1)
	if err != nil {
		t.Fatalf("ReadFile() file1 error = %v", err)
	}

	// Parse the second file's JSON stream message
	lines = strings.Split(strings.TrimSpace(string(fileData)), "\n")
	if len(lines) != 1 {
		t.Fatalf("Expected 1 line in file1, got %d", len(lines))
	}
	var sm2 StreamMessage
	if err := json.Unmarshal([]byte(lines[0]), &sm2); err != nil {
		t.Fatalf("Failed to unmarshal stream message from file1: %v", err)
	}
	if sm2.Val != data2 {
		t.Errorf("file1 stream message value = %q, want %q", sm2.Val, data2)
	}
}

func TestFileLogManager_RetentionCleanup(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := FileLogManagerCfg{
		LogDir:        tmpDir,
		ScanInterval:  50 * time.Millisecond,
		RetentionTime: 100 * time.Millisecond,
		MaxSizeBytes:  0,
	}

	manager := NewFileLogManager(cfg)

	logger, err := manager.NewLogger("test-exec")
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	_, err = logger.Write([]byte("test data\n"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(files) == 0 {
		t.Fatal("Expected log file to be created")
	}

	logger.Close()
	time.Sleep(150 * time.Millisecond)
	ctx := context.Background()
	logger_slog := slog.Default()

	err = manager.(*FileLogManager).run(ctx, logger_slog)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	files, err = os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	for _, file := range files {
		if !file.IsDir() {
			t.Errorf("Expected old log file to be deleted, but found: %s", file.Name())
		}
	}
}

func TestFileLogManager_ActiveLoggerProtection(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := FileLogManagerCfg{
		LogDir:        tmpDir,
		ScanInterval:  50 * time.Millisecond,
		RetentionTime: 50 * time.Millisecond,
		MaxSizeBytes:  0,
	}

	manager := NewFileLogManager(cfg)

	logger, err := manager.NewLogger("test-exec")
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	_, err = logger.Write([]byte("test data\n"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	ctx := context.Background()
	logger_slog := slog.Default()

	err = manager.(*FileLogManager).run(ctx, logger_slog)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	found := false
	for _, file := range files {
		if !file.IsDir() {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected log file to be protected from deletion while logger is active")
	}
}

func TestFileLogManager_isFileInUse(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := FileLogManagerCfg{
		LogDir:        tmpDir,
		ScanInterval:  50 * time.Millisecond,
		RetentionTime: 1 * time.Hour,
		MaxSizeBytes:  0,
	}

	manager := NewFileLogManager(cfg).(*FileLogManager)

	logger, err := manager.NewLogger("test-exec")
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	if !manager.isFileInUse("test-exec.0") {
		t.Error("isFileInUse() = false, want true for active logger")
	}

	logger.Close()

	if manager.isFileInUse("test-exec.0") {
		t.Error("isFileInUse() = true, want false for closed logger")
	}

	if manager.isFileInUse("non-existent.0") {
		t.Error("isFileInUse() = true, want false for non-existent logger")
	}
	if manager.isFileInUse("invalid-filename") {
		t.Error("isFileInUse() = true, want false for invalid filename")
	}
}

func TestFileLogManager_StreamLogs_ArchivedOnly(t *testing.T) {
	tmpDir := t.TempDir()
	execID := "test-exec"

	cfg := FileLogManagerCfg{
		LogDir:       tmpDir,
		ScanInterval: 1 * time.Hour, // Don't run cleanup during test
		MaxSizeBytes: 10,            // Small size to force rotation
	}

	manager := NewFileLogManager(cfg).(*FileLogManager)

	// Create a logger and write data that causes rotation
	logger, err := manager.NewLogger(execID)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	// Write data to create multiple files
	data1 := "line1\nline2\n"
	data2 := "line3\nline4\n"

	logger.Write([]byte(data1))
	time.Sleep(100 * time.Millisecond) // Let it sync

	logger.Write([]byte(data2))
	time.Sleep(100 * time.Millisecond) // Let it sync and rotate

	// Close the logger so it becomes archived
	logger.Close()

	// Now stream the logs
	ctx := context.Background()
	logCh, err := manager.StreamLogs(ctx, execID, make(map[string]int32))
	if err != nil {
		t.Fatalf("StreamLogs() error = %v", err)
	}

	var jsonMessages []string
	for jsonMsg := range logCh {
		jsonMessages = append(jsonMessages, jsonMsg)
	}

	// We expect 2 JSON messages (one for each Write call)
	if len(jsonMessages) != 2 {
		t.Fatalf("Expected 2 JSON messages, got %d: %v", len(jsonMessages), jsonMessages)
	}

	// Parse and verify each JSON message
	for i, jsonMsg := range jsonMessages {
		var sm StreamMessage
		if err := json.Unmarshal([]byte(jsonMsg), &sm); err != nil {
			t.Fatalf("Failed to unmarshal JSON message %d: %v", i, err)
		}
		expectedData := []string{data1, data2}
		if sm.Val != expectedData[i] {
			t.Errorf("JSON message %d: got %q, want %q", i, sm.Val, expectedData[i])
		}
		if sm.MType != LogMessageType {
			t.Errorf("JSON message %d type: got %v, want %v", i, sm.MType, LogMessageType)
		}
	}
}

func TestFileLogManager_StreamLogs_ActiveLogger(t *testing.T) {
	tmpDir := t.TempDir()
	execID := "test-exec-active"

	cfg := FileLogManagerCfg{
		LogDir:       tmpDir,
		ScanInterval: 1 * time.Hour,
		MaxSizeBytes: 0, // No rotation
	}

	manager := NewFileLogManager(cfg).(*FileLogManager)

	// Create an active logger
	logger, err := manager.NewLogger(execID)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	// Write initial data
	initialData := "initial line\n"
	logger.Write([]byte(initialData))
	time.Sleep(100 * time.Millisecond)

	// Start streaming
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logCh, err := manager.StreamLogs(ctx, execID, make(map[string]int32))
	if err != nil {
		t.Fatalf("StreamLogs() error = %v", err)
	}

	// Read initial JSON message
	jsonMsg := <-logCh
	var sm StreamMessage
	if err := json.Unmarshal([]byte(jsonMsg), &sm); err != nil {
		t.Fatalf("Failed to unmarshal initial stream message: %v", err)
	}
	if sm.Val != initialData {
		t.Errorf("Expected %q, got %q", initialData, sm.Val)
	}

	// Write more data while streaming
	go func() {
		time.Sleep(50 * time.Millisecond)
		logger.Write([]byte("new line\n"))
		time.Sleep(150 * time.Millisecond) // Let it sync
	}()

	// Should receive the new JSON message
	jsonMsg = <-logCh
	if err := json.Unmarshal([]byte(jsonMsg), &sm); err != nil {
		t.Fatalf("Failed to unmarshal new stream message: %v", err)
	}
	if sm.Val != "new line\n" {
		t.Errorf("Expected %q, got %q", "new line\n", sm.Val)
	}
}

func TestFileLogManager_StreamLogs_FollowsRotation(t *testing.T) {
	tmpDir := t.TempDir()
	execID := "test-exec-live-rotation"
	manager := NewFileLogManager(FileLogManagerCfg{
		LogDir:       tmpDir,
		ScanInterval: time.Hour,
		MaxSizeBytes: 1,
	}).(*FileLogManager)

	logger, err := manager.NewLogger(execID)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	fl := logger.(*FileLogger)
	defer logger.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logCh, err := manager.StreamLogs(ctx, execID, nil)
	if err != nil {
		t.Fatalf("StreamLogs() error = %v", err)
	}

	if _, err := logger.Write([]byte("before rotation\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := fl.filesync(); err != nil {
		t.Fatalf("filesync() error = %v", err)
	}
	assertStreamValue(t, logCh, "before rotation\n")

	if _, err := logger.Write([]byte("after rotation\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := fl.filesync(); err != nil {
		t.Fatalf("rotating filesync() error = %v", err)
	}
	assertStreamValue(t, logCh, "after rotation\n")
}

func TestFileLogManager_StreamLogs_HoldsPartialLine(t *testing.T) {
	tmpDir := t.TempDir()
	execID := "test-exec-partial"
	manager := NewFileLogManager(FileLogManagerCfg{LogDir: tmpDir, ScanInterval: time.Hour}).(*FileLogManager)
	closed := make(chan struct{})
	logCh := make(chan string)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		defer close(logCh)
		if err := manager.followLogs(ctx, execID, closed, nil, logCh); err != nil && ctx.Err() == nil {
			t.Errorf("followLogs() error = %v", err)
		}
	}()

	path := filepath.Join(tmpDir, execID+".0")
	message := `{"action_id":"a1","message_type":"log","node_id":"n1","value":"complete","timestamp":"2026-08-12T10:04:11Z","retry":1}`
	split := len(message) / 2
	if err := os.WriteFile(path, []byte(message[:split]), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	select {
	case got := <-logCh:
		t.Fatalf("received partial line %q", got)
	case <-time.After(2 * LogPollInterval):
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	if _, err := f.WriteString(message[split:] + "\n"); err != nil {
		f.Close()
		t.Fatalf("WriteString() error = %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case got := <-logCh:
		if got != message {
			t.Fatalf("got %q, want %q", got, message)
		}
	case <-time.After(time.Second):
		t.Fatal("complete line was not streamed")
	}
	close(closed)
	select {
	case _, ok := <-logCh:
		if ok {
			t.Fatal("stream remained open after logger close")
		}
	case <-time.After(time.Second):
		t.Fatal("stream did not close")
	}
}

func TestFileLogManager_StreamLogs_LoggerCloseDrains(t *testing.T) {
	tmpDir := t.TempDir()
	execID := "test-exec-close-drain"
	manager := NewFileLogManager(FileLogManagerCfg{LogDir: tmpDir, ScanInterval: time.Hour}).(*FileLogManager)
	logger, err := manager.NewLogger(execID)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	logCh, err := manager.StreamLogs(context.Background(), execID, nil)
	if err != nil {
		t.Fatalf("StreamLogs() error = %v", err)
	}
	if _, err := logger.Write([]byte("last buffered record\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	assertStreamValue(t, logCh, "last buffered record\n")
	select {
	case _, ok := <-logCh:
		if ok {
			t.Fatal("stream emitted an unexpected extra message")
		}
	case <-time.After(time.Second):
		t.Fatal("stream hung after logger close")
	}
}

func assertStreamValue(t *testing.T, logCh <-chan string, want string) {
	t.Helper()
	select {
	case raw, ok := <-logCh:
		if !ok {
			t.Fatal("log stream closed before a message arrived")
		}
		var msg StreamMessage
		if err := json.Unmarshal([]byte(raw), &msg); err != nil {
			t.Fatalf("unmarshal stream message: %v", err)
		}
		if msg.Val != want {
			t.Fatalf("stream value = %q, want %q", msg.Val, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %q", want)
	}
}

func TestFileLogManager_StreamLogs_MultipleRotatedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	execID := "test-exec-rotate"

	cfg := FileLogManagerCfg{
		LogDir:       tmpDir,
		ScanInterval: 1 * time.Hour,
		MaxSizeBytes: 5, // Very small to force multiple rotations
	}

	manager := NewFileLogManager(cfg).(*FileLogManager)

	logger, err := manager.NewLogger(execID)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	// Write data that will create multiple files
	writes := []string{"AAAAAAA\n", "BBBBBBB\n", "CCCCCCC\n", "DDDDDDD\n", "EEEEEEE\n"}
	for _, data := range writes {
		logger.Write([]byte(data))
		time.Sleep(120 * time.Millisecond) // Let it sync and potentially rotate
	}

	logger.Close()

	// Stream all logs
	ctx := context.Background()
	logCh, err := manager.StreamLogs(ctx, execID, make(map[string]int32))
	if err != nil {
		t.Fatalf("StreamLogs() error = %v", err)
	}

	var jsonMessages []string
	for jsonMsg := range logCh {
		jsonMessages = append(jsonMessages, jsonMsg)
	}

	expected := []string{"AAAAAAA\n", "BBBBBBB\n", "CCCCCCC\n", "DDDDDDD\n", "EEEEEEE\n"}
	if len(jsonMessages) != len(expected) {
		t.Fatalf("Expected %d JSON messages, got %d: %v", len(expected), len(jsonMessages), jsonMessages)
	}

	// Parse and verify each JSON message
	for i, jsonMsg := range jsonMessages {
		var sm StreamMessage
		if err := json.Unmarshal([]byte(jsonMsg), &sm); err != nil {
			t.Fatalf("Failed to unmarshal JSON message %d: %v", i, err)
		}
		if sm.Val != expected[i] {
			t.Errorf("JSON message %d: got %q, want %q", i, sm.Val, expected[i])
		}
		if sm.MType != LogMessageType {
			t.Errorf("JSON message %d type: got %v, want %v", i, sm.MType, LogMessageType)
		}
	}
}

func TestFileLogManager_StreamLogs_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	execID := "test-exec-cancel"

	cfg := FileLogManagerCfg{
		LogDir:       tmpDir,
		ScanInterval: 1 * time.Hour,
		MaxSizeBytes: 0,
	}

	manager := NewFileLogManager(cfg).(*FileLogManager)

	logger, err := manager.NewLogger(execID)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Close()

	logger.Write([]byte("test line\n"))
	time.Sleep(100 * time.Millisecond)

	// Create context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	logCh, err := manager.StreamLogs(ctx, execID, make(map[string]int32))
	if err != nil {
		t.Fatalf("StreamLogs() error = %v", err)
	}

	// Read one JSON message
	jsonMsg := <-logCh
	var sm StreamMessage
	if err := json.Unmarshal([]byte(jsonMsg), &sm); err != nil {
		t.Fatalf("Failed to unmarshal stream message: %v", err)
	}
	if sm.Val != "test line\n" {
		t.Errorf("Expected %q, got %q", "test line\n", sm.Val)
	}

	// Cancel context
	cancel()

	// Channel should close due to context cancellation
	select {
	case _, ok := <-logCh:
		if ok {
			t.Error("Expected channel to be closed after context cancellation")
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Channel did not close after context cancellation")
	}
}

func TestFileLogManager_StreamLogs_NonExistentExecID(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := FileLogManagerCfg{
		LogDir:       tmpDir,
		ScanInterval: 1 * time.Hour,
		MaxSizeBytes: 0,
	}

	manager := NewFileLogManager(cfg).(*FileLogManager)

	ctx := context.Background()
	logCh, err := manager.StreamLogs(ctx, "non-existent-exec", make(map[string]int32))
	if err != nil {
		t.Fatalf("StreamLogs() error = %v", err)
	}

	// Should receive no JSON messages and channel should close immediately
	select {
	case jsonMsg, ok := <-logCh:
		if ok {
			t.Errorf("Expected no JSON messages for non-existent exec, got: %q", jsonMsg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Channel did not close for non-existent exec ID")
	}
}

func TestFileLogManager_LoggerExists(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := FileLogManagerCfg{
		LogDir:       tmpDir,
		ScanInterval: 1 * time.Hour,
		MaxSizeBytes: 0,
	}

	manager := NewFileLogManager(cfg)

	// Test non-existent logger
	if manager.LoggerExists("non-existent") {
		t.Error("LoggerExists() = true, want false for non-existent logger")
	}

	// Create a logger
	logger, err := manager.NewLogger("test-exec")
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	// Test active logger
	if !manager.LoggerExists("test-exec") {
		t.Error("LoggerExists() = false, want true for active logger")
	}

	// Close the logger
	logger.Close()

	// Test closed logger
	if manager.LoggerExists("test-exec") {
		t.Error("LoggerExists() = true, want false for closed logger")
	}
}

func TestExtractFileIndex(t *testing.T) {
	tests := []struct {
		filename string
		expected int
	}{
		{"exec-123.0", 0},
		{"exec-123.1", 1},
		{"exec-123.42", 42},
		{"exec-123", 0},     // No dot
		{"exec-123.abc", 0}, // Invalid number
		{"exec-123.", 0},    // Empty after dot
		{".123", 123},       // Dot at start
		{"", 0},             // Empty string
	}

	for _, test := range tests {
		result := extractFileIndex(test.filename)
		if result != test.expected {
			t.Errorf("extractFileIndex(%q) = %d, want %d", test.filename, result, test.expected)
		}
	}
}
