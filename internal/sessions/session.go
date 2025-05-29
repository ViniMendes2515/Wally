package sessions

import "sync"

var (
	stateMap = make(map[string]string)
	mu       sync.RWMutex
)

func Set(user string, state string) {
	mu.Lock()
	defer mu.Unlock()
	stateMap[user] = state
}

func Get(user string) string {
	mu.RLock()
	defer mu.RUnlock()
	return stateMap[user]
}

func Delete(user string) {
	mu.Lock()
	defer mu.Unlock()
	delete(stateMap, user)
}
