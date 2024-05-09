package keymanager

import (
	"context"
	"fmt"
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
