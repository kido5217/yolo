package theme

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// KV is the TUI key-value store (port of context/kv.tsx): a flat
// JSON map[string]any persisted at path, read `??`-style, written
// ordered + atomic + cross-process locked. The engine is the only
// consumer in S0.
//
// Concurrency model: the in-memory store is the source of truth
// (updated under mu at Set time); the queue only carries flush
// triggers, so a single writer goroutine applies them in order
// (the upstream promise-chain port). The queue is NEVER closed
// (a Set racing Close would panic on a closed channel); Close
// instead closes done, the writer drains the remaining triggers,
// performs one final flush, and exits.
type KV struct {
	path string

	mu       sync.Mutex
	closed   bool
	store    map[string]any
	queue    chan struct{}
	done     chan struct{} // Close closes it; the writer selects on it
	finished chan struct{}
	once     sync.Once
}

// OpenKV opens (creating, with parent dirs) the KV file at path. A
// missing file yields an empty store; a corrupt file is logged and
// yields an empty store (upstream catch → console.error → continue);
// the only error is an unwritable parent dir.
func OpenKV(path string) (*KV, error) {
	kv := &KV{
		path:     path,
		store:    map[string]any{},
		queue:    make(chan struct{}, 1024),
		done:     make(chan struct{}),
		finished: make(chan struct{}),
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("theme: kv: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("theme: kv: %w", err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &kv.store); err != nil {
			slog.Warn("theme: kv: corrupt file, starting empty", "path", path, "error", err)
			kv.store = map[string]any{}
		}
	}
	go kv.run()
	return kv, nil
}

// Get returns store[key], or def when the key is absent or its value
// is nil — `??` semantics: JSON falsy values (false, "", 0) are
// preserved.
func (k *KV) Get(key string, def any) any {
	k.mu.Lock()
	defer k.mu.Unlock()
	if v, ok := k.store[key]; ok && v != nil {
		return v
	}
	return def
}

// Set stores val under key (a nil val deletes the key) and requests a
// flush. The store is updated under the lock; a flush trigger is then
// offered to the single writer (the upstream promise-chain port), which
// serializes + flock-locks + atomically rewrites the file. Offer is
// non-blocking: if 1024 triggers are already queued the writer is
// guaranteed to flush on its next tick, so dropping a trigger loses no
// state (the store already holds it). Set after Close is a no-op.
func (k *KV) Set(key string, val any) {
	k.mu.Lock()
	if k.closed {
		k.mu.Unlock()
		return
	}
	if val == nil {
		delete(k.store, key)
	} else {
		k.store[key] = val
	}
	select {
	case k.queue <- struct{}{}:
	default:
	}
	k.mu.Unlock()
}

// Close marks the store closed, stops the writer, and is idempotent.
// Because the queue is never closed, an in-flight Set cannot panic on a
// closed channel: the writer exits via done, and its final drain+flush
// captures every Set that offered before Close returned.
func (k *KV) Close() error {
	k.once.Do(func() {
		k.mu.Lock()
		k.closed = true
		k.mu.Unlock()
		close(k.done)
		<-k.finished
	})
	return nil
}

// run is the single writer goroutine (the upstream promise-chain
// port): each queued trigger flushes the whole store, ordered. On
// done it drains any remaining triggers and performs a final flush so
// nothing offered before Close is lost.
func (k *KV) run() {
	defer close(k.finished)
	flush := func() {
		k.mu.Lock()
		defer k.mu.Unlock()
		k.writeLocked()
	}
	for {
		select {
		case <-k.queue:
			flush()
		case <-k.done:
			for {
				select {
				case <-k.queue:
				default:
					flush()
					return
				}
			}
		}
	}
}

// writeLocked marshals + atomically rewrites the store; the caller
// holds k.mu.
func (k *KV) writeLocked() {
	data, err := json.Marshal(k.store)
	if err != nil {
		slog.Warn("theme: kv: marshal failed, write dropped", "error", err)
		return
	}
	if err := k.writeAtomic(data); err != nil {
		slog.Warn("theme: kv: write failed", "path", k.path, "error", err)
	}
}

// writeAtomic writes data to path via temp-file + rename, holding an
// exclusive flock on the target file for the whole write (upstream
// flockfile, kv.tsx:96-111; POSIX — Linux platform scope).
func (k *KV) writeAtomic(data []byte) error {
	dir := filepath.Dir(k.path)
	tmp, err := os.CreateTemp(dir, ".kv-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	lockF, err := os.OpenFile(k.path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer lockF.Close()
	if err := syscall.Flock(int(lockF.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer func() { _ = syscall.Flock(int(lockF.Fd()), syscall.LOCK_UN) }()
	if err := os.Rename(tmpName, k.path); err != nil {
		return err
	}
	return nil
}
