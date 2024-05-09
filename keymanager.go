package keymanager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	LocalKeyManagerType = "local"

	BackupKeysFileName    = "key_backup.txt"
	BackupLastKeyFileName = "last_key_backup.txt"
)

// Key represents a key with its associated metadata.
type Key struct {
	KeyID     string    // The unique identifier of the key.
	KeyValue  string    // The value of the key.
	CreatedAt time.Time // The timestamp when the key was created.
	ExpiresAt time.Time // The timestamp when the key expires.
}

// KeyManager is an interface that defines the methods for managing keys.
type KeyManager interface {
	GetKey(keyID string) (*Key, error) // GetKey retrieves the key associated with the given keyID.
	GetLastKey() (*Key, error)         // GetLastKey returns the last key stored in the KeyManager.
}

// Opts represents the options for creating a KeyManager.
type Opts struct {
	Type                string               // The type of the KeyManager.
	LocalKeyManagerOpts *LocalKeyManagerOpts // The options for creating a LocalKeyManager.
}

// NewKeyManager creates a new KeyManager based on the provided options.
// It returns an instance of KeyManager and an error, if any.
func NewKeyManager(ctx context.Context, opts *Opts) (KeyManager, error) {
	switch opts.Type {
	case LocalKeyManagerType:
		return NewLocalKeyManager(ctx, opts.LocalKeyManagerOpts)
	default:
		return nil, fmt.Errorf("unsupported key manager type: %s", opts.Type)
	}
}

// LocalKeyManager is a key manager implementation that stores keys locally in memory.
type LocalKeyManager struct {
	keys         map[string]*Key // The map of keys stored in the LocalKeyManager.
	lastKey      *Key            // The last key stored in the LocalKeyManager.
	keyCount     int             // The desired count of keys to be stored.
	keyLifeTime  time.Duration   // The lifetime of each key.
	flushEnabled bool            // The flag to enable/disable flushing of keys.
	m            sync.Mutex      // The mutex for synchronizing access to the keys.
}

// LocalKeyManagerOpts represents the options for creating a LocalKeyManager.
type LocalKeyManagerOpts struct {
	KeyCount              int           // The desired count of keys to be stored.
	KeyLifeTime           time.Duration // The lifetime of each key.
	BackgroundJobInterval time.Duration // The interval at which the background job runs.
	FlushEnabled          bool          // The flag to enable/disable flushing of keys.
	BackupFilePath        string        // The file path for backing up keys.
}

// NewLocalKeyManager creates a new instance of LocalKeyManager.
// It initializes the key manager with the given options and starts a background job to manage the keys.
// The background job runs at the specified interval in the options.
// Returns a pointer to the created LocalKeyManager and any error encountered during initialization.
func NewLocalKeyManager(ctx context.Context, opts *LocalKeyManagerOpts) (*LocalKeyManager, error) {
	keyManager := &LocalKeyManager{
		keys:         make(map[string]*Key),
		lastKey:      nil,
		keyCount:     opts.KeyCount,
		keyLifeTime:  opts.KeyLifeTime,
		flushEnabled: opts.FlushEnabled,
		m:            sync.Mutex{},
	}

	if opts.BackupFilePath != "" {
		keyManager.lastKey = &Key{}
		if err := keyManager.loadKeys(opts.BackupFilePath); err != nil {
			return nil, err
		}
	} else {
		keyManager.createKeys()
	}
	go keyManager.run(opts.BackgroundJobInterval)

	return keyManager, nil
}

// run is a background job that runs at the specified interval and performs key management tasks.
func (l *LocalKeyManager) run(backgroundJobInterval time.Duration) {
	ticker := time.NewTicker(backgroundJobInterval)
	defer ticker.Stop()

	for range ticker.C {
		l.deleteExpiredKeys()
		l.createKeys()
		if l.flushEnabled {
			l.flushKeys()
		}
	}
}

// deleteExpiredKeys deletes the expired keys from the LocalKeyManager.
// It iterates over the keys stored in the LocalKeyManager and removes any key
// that has expired based on the current time and the key's expiration time.
func (l *LocalKeyManager) deleteExpiredKeys() {
	l.m.Lock()
	defer l.m.Unlock()

	for keyID, key := range l.keys {
		if key.ExpiresAt.Before(time.Now().Add(-l.keyLifeTime)) {
			if l.lastKey == key {
				l.lastKey = nil
			}
			delete(l.keys, keyID)
		}
	}
}

// createKeys generates new keys and adds them to the key manager until the desired key count is reached.
func (l *LocalKeyManager) createKeys() {
	l.m.Lock()
	defer l.m.Unlock()

	for len(l.keys) < l.keyCount {
		key := &Key{
			KeyID:     uuid.New().String(),
			KeyValue:  uuid.New().String(),
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(l.keyLifeTime),
		}
		l.keys[key.KeyID] = key
		l.lastKey = key
	}
}

// getKeyList returns a list of keys stored in the LocalKeyManager.
func (l *LocalKeyManager) getKeyList() []*Key {
	l.m.Lock()
	defer l.m.Unlock()

	keys := make([]*Key, len(l.keys))
	idx := 0
	for _, key := range l.keys {
		keys[idx] = key
		idx++
	}

	return keys
}

// flushKeys writes the key list and the last key to backup files.
func (l *LocalKeyManager) flushKeys() {
	keys := l.getKeyList()
	jsonKeys, _ := json.Marshal(keys)
	jsonLastKey, _ := json.Marshal(l.lastKey)
	_ = os.WriteFile(BackupKeysFileName, jsonKeys, 0644)
	_ = os.WriteFile(BackupLastKeyFileName, jsonLastKey, 0644)
}

// loadKeys loads the backup keys and last key from the specified base path.
// It reads the backup keys and last key from the corresponding files in the base path,
// unmarshals them into the appropriate data structures, and updates the local key manager's keys.
// If any error occurs during the process, it is returned.
func (l *LocalKeyManager) loadKeys(basePath string) error {
	backupKeys, err := os.ReadFile(path.Join(basePath, BackupKeysFileName))
	if err != nil {
		return err
	}
	backupLastKey, err := os.ReadFile(path.Join(basePath, BackupLastKeyFileName))
	if err != nil {
		return err
	}

	keyList := make([]*Key, len(l.keys))
	if err = json.Unmarshal(backupKeys, &keyList); err != nil {
		return err
	}
	if err = json.Unmarshal(backupLastKey, &l.lastKey); err != nil {
		return err
	}
	for _, key := range keyList {
		l.keys[key.KeyID] = key
	}

	return nil
}

// GetKey retrieves the key associated with the given keyID from the local key manager.
// If the key is found, it returns the key and a nil error. If the key is not found,
// it returns nil and an error indicating that the key was not found.
func (l *LocalKeyManager) GetKey(keyID string) (*Key, error) {
	l.m.Lock()
	defer l.m.Unlock()

	key, ok := l.keys[keyID]
	if !ok {
		return nil, fmt.Errorf("key not found. keyID: %s", keyID)
	}

	return key, nil
}

// GetLastKey returns the last key stored in the LocalKeyManager.
// If no keys are available, it returns an error.
func (l *LocalKeyManager) GetLastKey() (*Key, error) {
	l.m.Lock()
	defer l.m.Unlock()

	if l.lastKey == nil {
		return nil, fmt.Errorf("no keys available")
	}
	return l.lastKey, nil
}
