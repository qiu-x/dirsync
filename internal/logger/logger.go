package logger

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"dirsync/internal/fslisten"
)

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	WorkerID  int    `json:"worker_id"`
	EventType string `json:"event_type"`
	EventPath string `json:"event_path"`
	Action    string `json:"action"`
}

type Logger struct {
	logFile string
	logChan chan LogEntry
	wg      sync.WaitGroup
}

func NewLogger(filename string) *Logger {
	l := &Logger{
		logFile: filename,
		logChan: make(chan LogEntry, 100), // Buffered channel
	}

	l.wg.Add(1)
	go l.writeLogWorker()

	return l
}

func (l *Logger) LogEvent(workerID int, event fslisten.Event, action string) {
	logEntry := LogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		WorkerID:  workerID,
		EventType: event.Type.String(),
		EventPath: event.Path,
		Action:    action,
	}
	l.logChan <- logEntry
}

func (l *Logger) writeLogWorker() {
	defer l.wg.Done()
	file, err := os.OpenFile(l.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal("Failed to open log file:", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)

	for entry := range l.logChan {
		if err := encoder.Encode(entry); err != nil {
			log.Println("Error writing log:", err)
		}
	}
}

func (l *Logger) Close() {
	close(l.logChan)
	l.wg.Wait()
}
