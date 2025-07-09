/*
Copyright 2025 The maco Authors

This program is offered under a commercial and under the AGPL license.
For AGPL licensing, see below.

AGPL licensing:
This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package dsutil

import (
	"sync"
)

type SafeHashMap[K comparable, V any] struct {
	rw sync.RWMutex
	m  map[K]V
}

func NewSafeHashMap[K comparable, V any]() *SafeHashMap[K, V] {
	return &SafeHashMap[K, V]{m: make(map[K]V)}
}

func (hm *SafeHashMap[K, V]) Set(k K, v V) {
	hm.rw.Lock()
	defer hm.rw.Unlock()
	hm.m[k] = v
}

func (hm *SafeHashMap[K, V]) Range(f func(K, V) bool) {
	hm.rw.RLock()
	defer hm.rw.RUnlock()
	for k, v := range hm.m {
		if !f(k, v) {
			break
		}
	}
}

func (hm *SafeHashMap[K, V]) Get(k K) (V, bool) {
	hm.rw.RLock()
	defer hm.rw.RUnlock()
	v, ok := hm.m[k]
	return v, ok
}

func (hm *SafeHashMap[K, V]) Contains(k K) bool {
	hm.rw.RLock()
	defer hm.rw.RUnlock()
	_, ok := hm.m[k]
	return ok
}

func (hm *SafeHashMap[K, V]) Remove(k K) {
	hm.rw.Lock()
	defer hm.rw.Unlock()
	delete(hm.m, k)
}

func (hm *SafeHashMap[K, V]) Keys() []K {
	keys := make([]K, 0, len(hm.m))
	hm.rw.RLock()
	defer hm.rw.RUnlock()
	for k := range hm.m {
		keys = append(keys, k)
	}
	return keys
}

func (hm *SafeHashMap[K, V]) Values() []V {
	values := make([]V, 0, len(hm.m))
	hm.rw.RLock()
	defer hm.rw.RUnlock()
	for _, v := range hm.m {
		values = append(values, v)
	}
	return values
}

func (hm *SafeHashMap[K, V]) Size() int {
	hm.rw.RLock()
	defer hm.rw.RUnlock()
	return len(hm.m)
}

func (hm *SafeHashMap[K, V]) Empty() bool {
	return hm.Size() == 0
}

func (hm *SafeHashMap[K, V]) Clear() {
	hm.rw.Lock()
	defer hm.rw.Unlock()
	hm.m = make(map[K]V)
}
