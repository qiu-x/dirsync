package worker

import (
	"context"
	"dirsync/internal/fslisten"
	"dirsync/internal/logger"
	"dirsync/internal/queue"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Worker struct {
	ID      int
	srcDir  string
	destDir string
	Queue   *queue.PersistentQueue[fslisten.Event]
	Wg      *sync.WaitGroup
	Ctx     context.Context
	Logger  *logger.Logger
}

func Copy(src string, destDir string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source file/directory: %w", err)
	}

	if srcInfo.IsDir() {
		return filepath.Walk(src, func(srcPath string, srcInfo os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("failed to walk through source directory: %w", err)
			}

			relPath, err := filepath.Rel(src, srcPath)
			if err != nil {
				return fmt.Errorf("failed to get relative path: %w", err)
			}

			destPath := filepath.Join(destDir, relPath)

			if srcInfo.IsDir() {
				return os.MkdirAll(destPath, os.ModePerm)
			}

			return CopyFile(srcPath, destPath)
		})
	}

	return CopyFile(src, destDir)
}

func CopyFile(src string, dest string) error {
	destInfo, err := os.Stat(dest)
	if err == nil && destInfo.IsDir() {
		return fmt.Errorf("destination %s is a directory, not a file", dest)
	}

	destDir := filepath.Dir(dest)
	destDirInfo, err := os.Stat(destDir)
	if err != nil || !destDirInfo.IsDir() {
		if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dest += ".bak"

	destFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}

func (w *Worker) processEvent(event fslisten.Event) {
	switch event.Type {
	case fslisten.Create:
	case fslisten.Modify: // There are cases the `Modify` could be handled more optimally, but this is also fine for now
		dst, err := filepath.Rel(w.srcDir, event.Path)
		if err != nil {
			log.Println("Sync Error:", err)
			os.Exit(1)
		}

		destPath := filepath.Join(w.destDir, dst)

		// If the filename starts with "delete_", remove it instead of copying
		if strings.HasPrefix(filepath.Base(dst), "delete_") {
			destDir, destBase := filepath.Split(destPath)
			destBase = strings.TrimPrefix(destBase, "delete_")
			w.deleteFileOrDir(event.Path, filepath.Join(destDir, destBase))
		} else {
			err = Copy(event.Path, destPath)
			if err != nil {
				log.Println("Sync Error:", err)
				os.Exit(1)
			}

			w.Logger.LogEvent(w.ID, event, fmt.Sprintf("Copied file to %s", filepath.Dir(destPath)))
		}
	}
}

func (w *Worker) deleteFileOrDir(srcPath, destPath string) {
	srcInfo, err := os.Stat(srcPath)
	if err == nil && !srcInfo.IsDir() {
		destPath += ".bak"
	}
	// Delete from hotdir
	if err := os.RemoveAll(srcPath); err != nil {
		log.Printf("Failed to delete source: %s, error: %v\n", srcPath, err)
	} else {
		w.Logger.LogEvent(w.ID, fslisten.Event{Path: srcPath, Type: fslisten.Delete}, "Deleted from source")
	}

	// Delete from backup
	if err := os.RemoveAll(destPath); err != nil {
		log.Printf("Failed to delete destination: %s, error: %v\n", destPath, err)
	} else {
		w.Logger.LogEvent(w.ID, fslisten.Event{Path: destPath, Type: fslisten.Delete}, "Deleted from destination")
	}

	fmt.Printf("Deleted: %s and %s\n", srcPath, destPath)
}


func (w *Worker) execute() {
	defer w.Wg.Done()
	for {
		select {
		case <-w.Ctx.Done():
			log.Printf("Worker %d shutting down: context canceled\n", w.ID)
			return
		default:
			event, ok := w.Queue.Pop()
			if ok {
				w.processEvent(event)
			}
		}
	}
}

func Spawn(
	ctx context.Context,
	q *queue.PersistentQueue[fslisten.Event],
	wg *sync.WaitGroup,
	id int,
	log *logger.Logger,
	srcDir, destDir string,
) {
	worker := &Worker{
		ID:      id,
		Queue:   q,
		Wg:      wg,
		Ctx:     ctx,
		Logger:  log,
		srcDir:  srcDir,
		destDir: destDir,
	}
	go worker.execute()
}
