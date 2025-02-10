package dirwatcher

import (
	"fmt"

	"dirsync/internal/fslisten"
)

type DirWatcher struct {
	path string
}

type EventHandler func(fslisten.Event, error)

func (d *DirWatcher) CreateEventHook() (func(EventHandler), error) {
	listener, err := fslisten.New()
	if err != nil {
		return nil, fmt.Errorf("error initializing inotify: %w", err)
	}

	if err := listener.WatchDirectory(d.path); err != nil {
		return nil, fmt.Errorf("error watching directory: %w", err)
	}

	return func(handler EventHandler) {
		for event, err := range listener.ReadEvents {
			fmt.Println("Event:", event.Type.String(), "on", event.Path)
			handler(event, err)
		}
		listener.Close()
	}, nil
}

func New(path string) *DirWatcher {
	return &DirWatcher{path}
}
