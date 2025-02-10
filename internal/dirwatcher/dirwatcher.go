package dirwatcher

import (
	"dirsync/internal/fslisten"
	"fmt"
)

type DirWatcher struct {
	path string
}

type EventHandler func(fslisten.Event, error)

func (d *DirWatcher) CreateEventHook() (func(EventHandler), error) {
	listener, err := fslisten.New()
	if err != nil {
		return nil, fmt.Errorf("Error initializing inotify: %v", err)
	}

	if err := listener.WatchDirectory(d.path); err != nil {
		return nil, fmt.Errorf("Error watching directory: %v", err)
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
