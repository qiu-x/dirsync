package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"dirsync/internal/dirwatcher"
	"dirsync/internal/fslisten"
	"dirsync/internal/logger"
	"dirsync/internal/queue"
	"dirsync/internal/worker"
)

func validateArgs(hotdir, backup, state string) error {
	if hotdir == "" || backup == "" || state == "" {
		return errors.New("--state, --hotdir and --backup arguments must be provided")
	}
	return nil
}

func checkDirExists(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("hotdir %s does not exist", path)
	}
	if err != nil {
		return fmt.Errorf("error checking hotdir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("hotdir %s is not a directory", path)
	}
	return nil
}

// Create the backup directory if it doesn't exist.
func ensureBackupDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		parent := filepath.Dir(path)
		if _, err := os.Stat(parent); os.IsNotExist(err) {
			return fmt.Errorf("parent directory %s does not exist", parent)
		}
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create backup directory: %w", err)
		}
		log.Println("Created backup directory:", path)
	} else if err != nil {
		return fmt.Errorf("error checking backup directory: %w", err)
	}
	return nil
}

func handleShutdown(cancel context.CancelFunc) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT)

	<-signalChan
	log.Println("Received termination signal. Shutting down gracefully...")
	cancel()
	time.Sleep(5 * time.Second)
	os.Exit(0)
}

func startSync(hotPath, backupPath, statePath, logPath string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handleShutdown(cancel)

	logger := logger.NewLogger(logPath)
	defer logger.Close()

	queue := queue.New[fslisten.Event](statePath)
	dw := dirwatcher.New(hotPath)
	eventHook, err := dw.CreateEventHook()
	if err != nil {
		log.Println("Error:", err)
		return err
	}

	go eventHook(func(ev fslisten.Event, err error) {
		if err != nil {
			log.Println("Error:", err)
			return
		}
		queue.PushBack(ev)
	})

	wg := sync.WaitGroup{}
	for i := range runtime.NumCPU() {
		worker.Spawn(ctx, queue, &wg, i, logger, hotPath, backupPath)
		wg.Add(1)
	}

	wg.Wait()
	return nil
}

func main() {
	hotdir := flag.String("hotdir", "", "Path to the hot directory")
	backup := flag.String("backup", "", "Path to the backup directory")
	state := flag.String("state", "", "Path to the internal state file")

	logFile := flag.String("log", "sync_log.json", "Path to the log file")
	viewMode := flag.Bool("view", false, "View log file and exit")

	// Command-line flags for filtering
	filter := flag.String("filter", "", "View Mode only. Filter logs by event path (partial match)")
	from := flag.String("from", "", "View Mode only. Filter logs from this datetime (RFC3339)")
	to := flag.String("to", "", "View Mode only. Filter logs up to this datetime (RFC3339)")

	flag.Parse()

	if *viewMode {
		viewer := logger.NewViewer(*logFile)

		// Get filtered logs
		logs, err := viewer.FilterLogs(*filter, *from, *to)
		if err != nil {
			log.Fatal("Error filtering logs:", err)
		}

		log.Printf("Showing %d matching log entries:\n\n", len(logs))
		logger.Print(logs)

		return
	}

	if err := validateArgs(*hotdir, *backup, *state); err != nil {
		log.Println("Error:", err)
		os.Exit(1)
	}

	if err := checkDirExists(*hotdir); err != nil {
		log.Println("Error:", err)
		os.Exit(1)
	}

	if err := ensureBackupDir(*backup); err != nil {
		log.Println("Error:", err)
		os.Exit(1)
	}

	if err := startSync(*hotdir, *backup, *state, *logFile); err != nil {
		log.Println("Error:", err)
		os.Exit(1)
	}
}
