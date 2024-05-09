package keymanager

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLocalKeyManager_logic(t *testing.T) {
	ctx := context.Background()
	LocalKeyManagerOpts := &LocalKeyManagerOpts{
		KeyCount:              5,
		KeyLifeTime:           10 * time.Second,
		FlushEnabled:          true,
		BackgroundJobInterval: 1 * time.Second,
		BackupFilePath:        "",
	}

	keyManager, err := NewLocalKeyManager(ctx, LocalKeyManagerOpts)
	assert.Empty(t, err)

	_, err = keyManager.GetLastKey()
	assert.Empty(t, err)

	time.Sleep(2 * time.Second)
}
