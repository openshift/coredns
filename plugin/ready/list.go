package ready

import (
	"sort"
	"strings"
	"sync"
)

// list is a structure that holds the plugins that signals readiness for this server block.
type list struct {
	sync.RWMutex
	rs    []Readiness
	names []string

	// keepReadiness indicates whether the readiness status of plugins should be retained
	// after they have been confirmed as ready. When set to false, the plugin readiness
	// status will be reset to nil to conserve resources, assuming ready plugins don't
	// need continuous monitoring.
	keepReadiness bool
}

// Reset resets l
func (l *list) Reset() {
	l.Lock()
	defer l.Unlock()
	l.rs = nil
	l.names = nil
}

// Append adds a new readiness to l.
func (l *list) Append(r Readiness, name string) {
	l.Lock()
	defer l.Unlock()
	l.rs = append(l.rs, r)
	l.names = append(l.names, name)
}

// Ready return true when all plugins ready, if the returned value is false the string
// contains a comma separated list of plugins that are not ready.
func (l *list) Ready() (bool, string) {
	l.RLock()
	defer l.RUnlock()
	ok := true
	s := []string{}
	for i, r := range l.rs {
		if r == nil {
			continue
		}
		if r.Ready() {
			if !l.keepReadiness {
				l.rs[i] = nil
			}
			continue
		}
		ok = false
		s = append(s, l.names[i])
	}
	if ok {
		return true, ""
	}
	sort.Strings(s)
	return false, strings.Join(s, ",")
}
