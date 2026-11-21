package cache

import (
	"strconv"
	"sync"
	"time"
	"uuid"
)

type pendingRename struct {
	sessionID uuid.UUID
	expiresAt time.Time
}

type InMemoryRenameCache struct {
	kvs map[string]pendingRename

	mu sync.Mutex
}

func NewInMemoryRenameCache() *InMemoryRenameCache {
	return &InMemoryRenameCache{
		kvs: make(map[string]pendingRename),
	}
}

func key(chatID int64, userID int64, promptID int) string {
	return strconv.FormatInt(chatID, 10) + ":" +
		strconv.FormatInt(userID, 10) + ":" +
		strconv.Itoa(promptID)
}

func (c *InMemoryRenameCache) Put(chatID int64, userID int64, promptID int, sessionID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.kvs[key(chatID, userID, promptID)] = pendingRename{
		sessionID: sessionID,
		expiresAt: time.Now().Add(time.Minute * 5),
	}
}

func (c *InMemoryRenameCache) Get(chatID int64, userID int64, promptID int) (uuid.UUID, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	k := key(chatID, userID, promptID)

	v, ok := c.kvs[k]
	if !ok {
		return uuid.Nil(), false
	}

	if v.expiresAt.Before(time.Now()) {
		delete(c.kvs, k)
		return uuid.Nil(), false
	}

	return v.sessionID, true
}

func (c *InMemoryRenameCache) Delete(chatID int64, userID int64, promptID int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.kvs, key(chatID, userID, promptID))
}
