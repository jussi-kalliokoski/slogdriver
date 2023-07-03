package slogtest

import (
	"encoding/json"
	"sync"
)

// Capture is an io.Writer that unmarshals the written data into entries of
// type T, to be later retrieved with Entries(). Written buffers must be valid
// JSON by themselves, and if the unmarshaling errors, Write will return an
// error.
type Capture[T any] struct {
	m       sync.Mutex
	entries []T
}

// Write implements io.Writer.
func (c *Capture[T]) Write(data []byte) (n int, err error) {
	n = len(data)

	var entry T
	if err = json.Unmarshal(data, &entry); err != nil {
		return n, err
	}

	c.m.Lock()
	defer c.m.Unlock()
	c.entries = append(c.entries, entry)

	return n, nil
}

// Entries returns the captured entries.
func (c *Capture[T]) Entries() []T {
	c.m.Lock()
	defer c.m.Unlock()
	return c.entries
}
