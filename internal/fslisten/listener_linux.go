package fslisten

import (
	"bytes"
	_ "encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

type EventType uint32

const (
	Delete EventType = iota
	Create
	Modify
	Ignore
)

func (t EventType) String() string {
	switch t {
	case Delete:
		return "Delete"
	case Create:
		return "Create"
	case Ignore:
		return "Ignore"
	}
	return "UNKNOWN"
}

func toEvetType(op uint32) EventType {
	switch {
	case op&unix.IN_CREATE == unix.IN_CREATE:
		return Create
	case op&unix.IN_MODIFY == unix.IN_MODIFY:
		return Create
	case op&unix.IN_DELETE == unix.IN_DELETE:
		return Delete
	case op&unix.IN_CLOSE_WRITE == unix.IN_CLOSE_WRITE:
		return Create
	case op&unix.IN_MOVED_TO == unix.IN_MOVED_TO:
		return Create
	case op&unix.IN_MOVED_FROM == unix.IN_MOVED_FROM:
		return Delete
	default:
		return Ignore
	}
}

type Event struct {
	Path string
	Type EventType
}

type Listener struct {
	fd      int
	watches map[int]string
	mu      sync.Mutex
}

func New() (*Listener, error) {
	fd, err := unix.InotifyInit1(0)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize inotify: %v", err)
	}

	w := &Listener{
		fd:      fd,
		watches: make(map[int]string),
	}

	return w, nil
}

func (w *Listener) Close() {
	unix.Close(w.fd)
}

func (w *Listener) WatchDirectory(path string) error {
	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return w.addWatch(p)
		}
		return nil
	})
	return err
}

func (w *Listener) ReadEvents(yield func(Event, error) bool) {
	var buf [unix.SizeofInotifyEvent + unix.NAME_MAX + 1]byte

	for {
		offset := 0

		n, err := unix.Read(w.fd, buf[:])
		if err != nil {
			if err == syscall.EAGAIN {
				continue
			}
			if !yield(Event{}, fmt.Errorf("error reading inotify events: %v", err)) {
				return
			}
		}

		for offset < n {
			e := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))

			nameBase := buf[offset+unix.SizeofInotifyEvent : offset+unix.SizeofInotifyEvent+int(e.Len)]
			name := string(bytes.TrimRight(nameBase, "\x00"))

			w.mu.Lock()
			path, exists := w.watches[int(e.Wd)]
			w.mu.Unlock()

			if exists {
				fullPath := filepath.Join(path, name)
				if !yield(Event{Path: fullPath, Type: toEvetType(e.Mask)}, nil) {
					return
				}

				// Handle newly created dirs
				if len(name) > 0 && e.Mask&unix.IN_ISDIR == unix.IN_ISDIR {
					// w.addWatch(fullPath)
					_ = w.WatchDirectory(fullPath)
				}
			}

			offset += int(unix.SizeofInotifyEvent + e.Len)
		}
	}
}

func (w *Listener) addWatch(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	wd, err := unix.InotifyAddWatch(w.fd, path, unix.IN_CREATE|unix.IN_DELETE|unix.IN_MODIFY|unix.IN_CLOSE_WRITE|unix.IN_MOVED_TO|unix.IN_MOVED_FROM)
	if err != nil {
		return fmt.Errorf("failed to add watch for %s: %v", path, err)
	}

	w.watches[wd] = path
	return nil
}

func (w *Listener) removeWatch(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	for wd, p := range w.watches {
		if p == path {
			if _, err := unix.InotifyRmWatch(w.fd, uint32(wd)); err != nil {
				return fmt.Errorf("failed to remove watch for %s: %v", path, err)
			}
			delete(w.watches, wd)
			return nil
		}
	}
	return fmt.Errorf("path %s not found in watches", path)
}
