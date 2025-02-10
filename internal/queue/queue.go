package queue

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"sync"
)

type PersistentQueue[T any] struct {
	mu       sync.Mutex
	cond     *sync.Cond // Used to signal when items are available
	items    []T
	filename string
}

func New[T any](filename string) *PersistentQueue[T] {
	q := &PersistentQueue[T]{items: make([]T, 0), filename: filename}
	q.cond = sync.NewCond(&q.mu)
	q.LoadFromFile()
	return q
}

func (q *PersistentQueue[T]) PushBack(item T) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, item)
	q.SaveToFile()
	q.cond.Signal()
}

func (q *PersistentQueue[T]) Pop() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		q.cond.Wait()
		var zero T
		return zero, false
	}
	item := q.items[0]
	q.items = q.items[1:]
	q.SaveToFile()
	return item, true
}

func (q *PersistentQueue[T]) SaveToFile() {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(q.items); err != nil {
		fmt.Println("Error saving queue:", err)
		return
	}
	if err := os.WriteFile(q.filename, buf.Bytes(), 0644); err != nil {
		fmt.Println("Error writing queue to file:", err)
	}
}

func (q *PersistentQueue[T]) LoadFromFile() {
	q.mu.Lock()
	defer q.mu.Unlock()
	data, err := os.ReadFile(q.filename)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet - it has to be created
			return
		}
		fmt.Println("Error loading queue from file:", err)
		return
	}
	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)
	if err := decoder.Decode(&q.items); err != nil {
		fmt.Println("Error decoding queue data:", err)
	}
}
