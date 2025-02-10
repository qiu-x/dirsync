package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

type LogViewer struct {
	logFile string
}

func NewViewer(filename string) *LogViewer {
	return &LogViewer{logFile: filename}
}

func (lv *LogViewer) LoadLogs() ([]LogEntry, error) {
	data, err := os.ReadFile(lv.logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	var logs []LogEntry
	decoder := json.NewDecoder(strings.NewReader(string(data)))

	for {
		var entry LogEntry
		if err := decoder.Decode(&entry); err != nil {
			break
		}
		logs = append(logs, entry)
	}

	return logs, err
}

func (lv *LogViewer) FilterLogs(filterRegex string, from, to string) ([]LogEntry, error) {
	logs, err := lv.LoadLogs()
	if err != nil {
		return nil, err
	}

	var regex *regexp.Regexp
	if filterRegex != "" {
		regex, err = regexp.Compile(filterRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
	}

	var filtered []LogEntry
	for _, log := range logs {
		logTime, err := time.Parse(time.RFC3339, log.Timestamp)
		if err != nil {
			continue
		}

		// match regex and check time range
		if (regex == nil || regex.MatchString(log.EventPath)) &&
			(isWithinTimeRange(logTime, from, to)) {
			filtered = append(filtered, log)
		}
	}

	return filtered, nil
}

func isWithinTimeRange(logTime time.Time, from, to string) bool {
	var fromTime, toTime time.Time
	var err error

	if from != "" {
		fromTime, err = time.Parse(time.RFC3339, from)
		if err != nil {
			return false
		}
		if logTime.Before(fromTime) {
			return false
		}
	}

	if to != "" {
		toTime, err = time.Parse(time.RFC3339, to)
		if err != nil {
			return false
		}
		if logTime.After(toTime) {
			return false
		}
	}

	return true
}

func Print(logs []LogEntry) {
	for _, log := range logs {
		fmt.Printf("[%s] Worker %d | %s | %s | %s\n",
			log.Timestamp, log.WorkerID, log.EventType, log.EventPath, log.Action)
	}
}
