package keymanager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLocalKeyManager_createKeys(t *testing.T) {
	l := &LocalKeyManager{
		keys:        make(map[string]*Key),
		keyCount:    5,
		keyLifeTime: time.Hour,
	}

	l.createKeys()

	assert.Equal(t, l.keyCount, len(l.keys))

	for _, key := range l.keys {
		assert.NotEmpty(t, key.KeyID)
		assert.NotEmpty(t, key.KeyValue)
		assert.NotZero(t, key.CreatedAt)
		assert.NotZero(t, key.ExpiresAt)
		assert.True(t, key.ExpiresAt.After(key.CreatedAt))
		assert.True(t, time.Now().Add(l.keyLifeTime).After(key.ExpiresAt))
	}
}

func TestLocalKeyManager_deleteExpiredKeys(t *testing.T) {
	l := &LocalKeyManager{
		keys:        make(map[string]*Key),
		keyCount:    5,
		keyLifeTime: time.Hour,
	}

	l.keys["1"] = &Key{
		ExpiresAt: time.Now().Add(-2 * time.Hour),
	}
	l.keys["2"] = &Key{
		ExpiresAt: time.Now().Add(-3 * time.Hour),
	}
	l.keys["3"] = &Key{
		ExpiresAt: time.Now(),
	}
	l.keys["4"] = &Key{
		ExpiresAt: time.Now().Add(-1 * time.Minute),
	}
	l.keys["5"] = &Key{
		ExpiresAt: time.Now().Add(-10 * time.Minute),
	}
	l.keys["6"] = &Key{
		ExpiresAt: time.Now().Add(-20 * time.Minute),
	}

	l.deleteExpiredKeys()

	assert.Equal(t, 4, len(l.keys))
	assert.NotEmpty(t, l.keys["3"])
	assert.NotEmpty(t, l.keys["4"])
	assert.NotEmpty(t, l.keys["5"])
	assert.NotEmpty(t, l.keys["6"])
}

func TestLocalKeyManager_GetKey(t *testing.T) {
	l := &LocalKeyManager{
		keys: make(map[string]*Key),
	}

	keyID := "testKey"
	key := &Key{
		KeyID:     keyID,
		KeyValue:  "testValue",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	l.keys[keyID] = key

	result, err := l.GetKey(keyID)
	assert.NoError(t, err)
	assert.Equal(t, key, result)
}

func TestLocalKeyManager_GetKey_KeyNotFound(t *testing.T) {
	l := &LocalKeyManager{
		keys: make(map[string]*Key),
	}

	keyID := "nonExistentKey"

	result, err := l.GetKey(keyID)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.EqualError(t, err, fmt.Sprintf("key not found. keyID: %s", keyID))
}

func TestLocalKeyManager_GetLastKey_NoKeysAvailable(t *testing.T) {
	l := &LocalKeyManager{
		keys: make(map[string]*Key),
	}

	result, err := l.GetLastKey()
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestLocalKeyManager_GetLastKey_WithKeysAvailable(t *testing.T) {
	l := &LocalKeyManager{
		keys: make(map[string]*Key),
		lastKey: &Key{
			KeyID:     "testKey",
			KeyValue:  "testValue",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}

	result, err := l.GetLastKey()
	assert.NoError(t, err)
	assert.Equal(t, l.lastKey, result)
}

func TestNewLocalKeyManager(t *testing.T) {
	ctx := context.Background()
	opts := &LocalKeyManagerOpts{
		KeyCount:              5,
		KeyLifeTime:           10 * time.Millisecond,
		BackgroundJobInterval: 100 * time.Millisecond,
	}

	keyManager, err := NewLocalKeyManager(ctx, opts)
	assert.NoError(t, err)
	assert.NotNil(t, keyManager)

	// Assert keyManager properties
	assert.Equal(t, opts.KeyCount, keyManager.keyCount)
	assert.Equal(t, opts.KeyLifeTime, keyManager.keyLifeTime)

	// Assert initial keys creation
	assert.Equal(t, opts.KeyCount, len(keyManager.keys))
	for _, key := range keyManager.keys {
		assert.NotEmpty(t, key.KeyID)
		assert.NotEmpty(t, key.KeyValue)
		assert.NotZero(t, key.CreatedAt)
		assert.NotZero(t, key.ExpiresAt)
		assert.True(t, key.ExpiresAt.After(key.CreatedAt))
		assert.True(t, time.Now().Add(keyManager.keyLifeTime).After(key.ExpiresAt))
	}

	lastKey := keyManager.lastKey

	// Assert background job is running
	time.Sleep(200 * time.Millisecond) // Wait for background job to run
	assert.NotEqual(t, lastKey, keyManager.lastKey)
}

func TestLocalKeyManager_getKeyMap(t *testing.T) {
	l := &LocalKeyManager{
		keys: make(map[string]*Key),
	}

	// Add some keys to the key manager
	key1 := &Key{
		KeyID:     "key1",
		KeyValue:  "value1",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	key2 := &Key{
		KeyID:     "key2",
		KeyValue:  "value2",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	l.keys["key1"] = key1
	l.keys["key2"] = key2

	// Call the getKeyList method
	result := l.getKeyMap()

	// Assert the length of the result
	assert.Len(t, result, 2)

	assert.Equal(t, result, l.keys)
}

func TestLocalKeyManager_flushKeys(t *testing.T) {
	defer cleanup(".")

	l := &LocalKeyManager{
		keys: make(map[string]*Key),
		lastKey: &Key{
			KeyID:    "testKey",
			KeyValue: "testValue",
		},
	}

	// Add some keys to the key manager
	key1 := &Key{
		KeyID:    "key1",
		KeyValue: "value1",
	}
	key2 := &Key{
		KeyID:    "key2",
		KeyValue: "value2",
	}
	l.keys["key1"] = key1
	l.keys["key2"] = key2

	// Call the flushKeys method
	l.flushKeys()

	// Assert the existence of the backup files
	_, err := os.Stat("key_backup.txt")
	assert.NoError(t, err)
	_, err = os.Stat("last_key_backup.txt")
	assert.NoError(t, err)

	// Read the backup files
	jsonKeys, err := os.ReadFile("key_backup.txt")
	assert.NoError(t, err)
	jsonLastKey, err := os.ReadFile("last_key_backup.txt")
	assert.NoError(t, err)

	// Unmarshal the backup data
	var backupKeys map[string]*Key
	err = json.Unmarshal(jsonKeys, &backupKeys)
	assert.NoError(t, err)
	var backupLastKey *Key
	err = json.Unmarshal(jsonLastKey, &backupLastKey)
	assert.NoError(t, err)

	// Assert the backup data
	assert.Len(t, backupKeys, 2)
	assert.Equal(t, backupKeys, l.keys)
	assert.Equal(t, l.lastKey, backupLastKey)
}

func TestLocalKeyManager_readKeys(t *testing.T) {
	defer cleanup(".")

	l := &LocalKeyManager{
		keys: make(map[string]*Key),
		lastKey: &Key{
			KeyID:    "testKey",
			KeyValue: "testValue",
		},
	}

	// Add some keys to the key manager
	key1 := &Key{
		KeyID:    "key1",
		KeyValue: "value1",
	}
	key2 := &Key{
		KeyID:    "key2",
		KeyValue: "value2",
	}
	l.keys["key1"] = key1
	l.keys["key2"] = key2

	// Call the flushKeys method
	l.flushKeys()

	// Assert the keys
	expectedKeys := l.keys
	expectedLastKey := l.lastKey

	l.keys = make(map[string]*Key)
	l.lastKey = &Key{}

	l.loadKeys(".")

	assert.Equal(t, expectedKeys, l.keys)
	assert.Equal(t, expectedLastKey, l.lastKey)
}

func cleanup(basePath string) {
	_ = os.Remove(path.Join(basePath, "key_backup.txt"))
	_ = os.Remove(path.Join(basePath, "last_key_backup.txt"))
}
