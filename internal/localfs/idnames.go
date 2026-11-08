package localfs

import (
	"os/user"
	"strconv"
	"sync"
)

// idNameCache maps a numeric id to its name. Names are looked up once per id for the process, so
// listings cost a map read per entry and every entry of one owner shares one string.
type idNameCache struct {
	mu     sync.RWMutex
	names  map[uint32]string
	lookup func(id string) (string, error)
}

func (c *idNameCache) name(id uint32) string {
	c.mu.RLock()
	name, ok := c.names[id]
	c.mu.RUnlock()
	if ok {
		return name
	}
	idStr := strconv.FormatUint(uint64(id), 10)
	name, err := c.lookup(idStr)
	if err != nil || name == "" {
		name = idStr
	}
	c.mu.Lock()
	c.names[id] = name
	c.mu.Unlock()
	return name
}

var (
	ownerNames = idNameCache{names: map[uint32]string{}, lookup: func(id string) (string, error) {
		u, err := user.LookupId(id)
		if err != nil {
			return "", err
		}
		return u.Username, nil
	}}
	groupNames = idNameCache{names: map[uint32]string{}, lookup: func(id string) (string, error) {
		g, err := user.LookupGroupId(id)
		if err != nil {
			return "", err
		}
		return g.Name, nil
	}}
)

func ownerName(uid uint32) string { return ownerNames.name(uid) }

func groupName(gid uint32) string { return groupNames.name(gid) }
