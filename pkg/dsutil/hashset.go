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

// Package hashset implements a set backed by a hash table.
//
// Structure is not thread safe.
//
// References: http://en.wikipedia.org/wiki/Set_%28abstract_data_type%29

package dsutil

import (
	"fmt"
	"strings"
)

// HashSet holds elements in go's native map
type HashSet[T comparable] struct {
	items map[T]struct{}
}

var itemExists = struct{}{}

// NewHashSet instantiates a new empty set and adds the passed values, if any, to the set
func NewHashSet[T comparable](values ...T) *HashSet[T] {
	set := &HashSet[T]{items: make(map[T]struct{})}
	if len(values) > 0 {
		set.Add(values...)
	}
	return set
}

// Add adds the items (one or more) to the set.
func (set *HashSet[T]) Add(items ...T) {
	for _, item := range items {
		set.items[item] = itemExists
	}
}

// Remove removes the items (one or more) from the set.
func (set *HashSet[T]) Remove(items ...T) {
	for _, item := range items {
		delete(set.items, item)
	}
}

// Contains check if items (one or more) are present in the set.
// All items have to be present in the set for the method to return true.
// Returns true if no arguments are passed at all, i.e. set is always superset of empty set.
func (set *HashSet[T]) Contains(items ...T) bool {
	for _, item := range items {
		if _, contains := set.items[item]; !contains {
			return false
		}
	}
	return true
}

// Empty returns true if set does not contain any elements.
func (set *HashSet[T]) Empty() bool {
	return set.Size() == 0
}

// Size returns number of elements within the set.
func (set *HashSet[T]) Size() int {
	return len(set.items)
}

// Clear clears all values in the set.
func (set *HashSet[T]) Clear() {
	set.items = make(map[T]struct{})
}

// Values returns all items in the set.
func (set *HashSet[T]) Values() []T {
	values := make([]T, set.Size())
	count := 0
	for item := range set.items {
		values[count] = item
		count++
	}
	return values
}

// String returns a string representation of container
func (set *HashSet[T]) String() string {
	str := "HashSet\n"
	items := []string{}
	for k := range set.items {
		items = append(items, fmt.Sprintf("%v", k))
	}
	str += strings.Join(items, ", ")
	return str
}

// Intersection returns the intersection between two sets.
// The new set consists of all elements that are both in "set" and "another".
// Ref: https://en.wikipedia.org/wiki/Intersection_(set_theory)
func (set *HashSet[T]) Intersection(another *HashSet[T]) *HashSet[T] {
	result := NewHashSet[T]()

	// Iterate over smaller set (optimization)
	if set.Size() <= another.Size() {
		for item := range set.items {
			if _, contains := another.items[item]; contains {
				result.Add(item)
			}
		}
	} else {
		for item := range another.items {
			if _, contains := set.items[item]; contains {
				result.Add(item)
			}
		}
	}

	return result
}

// Union returns the union of two sets.
// The new set consists of all elements that are in "set" or "another" (possibly both).
// Ref: https://en.wikipedia.org/wiki/Union_(set_theory)
func (set *HashSet[T]) Union(another *HashSet[T]) *HashSet[T] {
	result := NewHashSet[T]()

	for item := range set.items {
		result.Add(item)
	}
	for item := range another.items {
		result.Add(item)
	}

	return result
}

// Difference returns the difference between two sets.
// The new set consists of all elements that are in "set" but not in "another".
// Ref: https://proofwiki.org/wiki/Definition:Set_Difference
func (set *HashSet[T]) Difference(another *HashSet[T]) *HashSet[T] {
	result := NewHashSet[T]()

	for item := range set.items {
		if _, contains := another.items[item]; !contains {
			result.Add(item)
		}
	}

	return result
}
