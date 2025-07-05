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
	"fmt"
	"slices"
	"strings"
	"sync"
)

// List holds the elements, where each element points to the next element
type List[T comparable] struct {
	first *element[T]
	last  *element[T]
	size  int
}

type element[T comparable] struct {
	value T
	next  *element[T]
}

// NewList instantiates a new list and adds the passed values, if any, to the list
func NewList[T comparable](values ...T) *List[T] {
	list := &List[T]{}
	if len(values) > 0 {
		list.Add(values...)
	}
	return list
}

// Add appends a value (one or more) at the end of the list (same as Append())
func (list *List[T]) Add(values ...T) {
	for _, value := range values {
		newElement := &element[T]{value: value}
		if list.size == 0 {
			list.first = newElement
			list.last = newElement
		} else {
			list.last.next = newElement
			list.last = newElement
		}
		list.size++
	}
}

// Append appends a value (one or more) at the end of the list (same as Add())
func (list *List[T]) Append(values ...T) {
	list.Add(values...)
}

// Prepend prepends a values (or more)
func (list *List[T]) Prepend(values ...T) {
	// in reverse to keep passed order i.e. ["c","d"] -> Prepend(["a","b"]) -> ["a","b","c",d"]
	for v := len(values) - 1; v >= 0; v-- {
		newElement := &element[T]{value: values[v], next: list.first}
		list.first = newElement
		if list.size == 0 {
			list.last = newElement
		}
		list.size++
	}
}

// Get returns the element at index.
// Second return parameter is true if index is within bounds of the array and array is not empty, otherwise false.
func (list *List[T]) Get(index int) (T, bool) {

	if !list.withinRange(index) {
		var t T
		return t, false
	}

	ele := list.first
	for e := 0; e != index; e, ele = e+1, ele.next {
	}

	return ele.value, true
}

// Remove removes the ele at the given index from the list.
func (list *List[T]) Remove(index int) {

	if !list.withinRange(index) {
		return
	}

	if list.size == 1 {
		list.Clear()
		return
	}

	var beforeElement *element[T]
	ele := list.first
	for e := 0; e != index; e, ele = e+1, ele.next {
		beforeElement = ele
	}

	if ele == list.first {
		list.first = ele.next
	}
	if ele == list.last {
		list.last = beforeElement
	}
	if beforeElement != nil {
		beforeElement.next = ele.next
	}

	ele = nil

	list.size--
}

// Contains checks if values (one or more) are present in the set.
// All values have to be present in the set for the method to return true.
// Performance time complexity of n^2.
// Returns true if no arguments are passed at all, i.e. set is always super-set of empty set.
func (list *List[T]) Contains(values ...T) bool {

	if len(values) == 0 {
		return true
	}
	if list.size == 0 {
		return false
	}
	for _, value := range values {
		found := false
		for ele := list.first; ele != nil; ele = ele.next {
			if ele.value == value {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Values returns all eles in the list.
func (list *List[T]) Values() []T {
	values := make([]T, list.size, list.size)
	for e, ele := 0, list.first; ele != nil; e, ele = e+1, ele.next {
		values[e] = ele.value
	}
	return values
}

// IndexOf returns index of provided ele
func (list *List[T]) IndexOf(value T) int {
	if list.size == 0 {
		return -1
	}
	for index, ele := range list.Values() {
		if ele == value {
			return index
		}
	}
	return -1
}

// Empty returns true if list does not contain any eles.
func (list *List[T]) Empty() bool {
	return list.size == 0
}

// Size returns number of eles within the list.
func (list *List[T]) Size() int {
	return list.size
}

// Clear removes all eles from the list.
func (list *List[T]) Clear() {
	list.size = 0
	list.first = nil
	list.last = nil
}

// Sort sort values (in-place) using.
func (list *List[T]) Sort(comparator Comparator[T]) {

	if list.size < 2 {
		return
	}

	values := list.Values()
	slices.SortFunc(values, comparator)

	list.Clear()

	list.Add(values...)

}

// Swap swaps values of two eles at the given indices.
func (list *List[T]) Swap(i, j int) {
	if list.withinRange(i) && list.withinRange(j) && i != j {
		var ele1, ele2 *element[T]
		for e, currentElement := 0, list.first; ele1 == nil || ele2 == nil; e, currentElement = e+1, currentElement.next {
			switch e {
			case i:
				ele1 = currentElement
			case j:
				ele2 = currentElement
			}
		}
		ele1.value, ele2.value = ele2.value, ele1.value
	}
}

// Insert inserts values at specified index position shifting the value at that position (if any) and any subsequent eles to the right.
// Does not do anything if position is negative or bigger than list's size
// Note: position equal to list's size is valid, i.e. append.
func (list *List[T]) Insert(index int, values ...T) {

	if !list.withinRange(index) {
		// Append
		if index == list.size {
			list.Add(values...)
		}
		return
	}

	list.size += len(values)

	var beforeElement *element[T]
	foundElement := list.first
	for e := 0; e != index; e, foundElement = e+1, foundElement.next {
		beforeElement = foundElement
	}

	if foundElement == list.first {
		oldNextElement := list.first
		for i, value := range values {
			newElement := &element[T]{value: value}
			if i == 0 {
				list.first = newElement
			} else {
				beforeElement.next = newElement
			}
			beforeElement = newElement
		}
		beforeElement.next = oldNextElement
	} else {
		oldNextElement := beforeElement.next
		for _, value := range values {
			newElement := &element[T]{value: value}
			beforeElement.next = newElement
			beforeElement = newElement
		}
		beforeElement.next = oldNextElement
	}
}

// Set value at specified index
// Does not do anything if position is negative or bigger than list's size
// Note: position equal to list's size is valid, i.e. append.
func (list *List[T]) Set(index int, value T) {

	if !list.withinRange(index) {
		// Append
		if index == list.size {
			list.Add(value)
		}
		return
	}

	foundElement := list.first
	for e := 0; e != index; {
		e, foundElement = e+1, foundElement.next
	}
	foundElement.value = value
}

// String returns a string representation of container
func (list *List[T]) String() string {
	str := "SinglyLinkedList\n"
	values := []string{}
	for ele := list.first; ele != nil; ele = ele.next {
		values = append(values, fmt.Sprintf("%v", ele.value))
	}
	str += strings.Join(values, ", ")
	return str
}

// Check that the index is within bounds of the list
func (list *List[T]) withinRange(index int) bool {
	return index >= 0 && index < list.size
}

// SafeList concurrent safe List
type SafeList[T comparable] struct {
	sync.RWMutex
	list *List[T]
}

func NewSafeList[T comparable](elements ...T) *SafeList[T] {
	return &SafeList[T]{list: NewList(elements...)}
}

func (list *SafeList[T]) Add(elements ...T) {
	list.Lock()
	defer list.Unlock()
	list.list.Add(elements...)
}

func (list *SafeList[T]) Append(elements ...T) {
	list.Lock()
	defer list.Unlock()
	list.list.Append(elements...)
}

func (list *SafeList[T]) Prepend(elements ...T) {
	list.Lock()
	defer list.Unlock()
	list.list.Prepend(elements...)
}

func (list *SafeList[T]) Get(index int) (T, bool) {
	list.RLock()
	defer list.RUnlock()
	return list.list.Get(index)
}

func (list *SafeList[T]) Remove(index int) {
	list.Lock()
	defer list.Unlock()
	list.list.Remove(index)
}

func (list *SafeList[T]) Contains(element T) bool {
	list.RLock()
	defer list.RUnlock()
	return list.list.Contains(element)
}

func (list *SafeList[T]) Values() []T {
	list.RLock()
	defer list.RUnlock()
	return list.list.Values()
}

func (list *SafeList[T]) IndexOf(value T) int {
	list.RLock()
	defer list.RUnlock()
	return list.list.IndexOf(value)
}

func (list *SafeList[T]) Empty() bool {
	list.RLock()
	defer list.RUnlock()
	return list.list.Empty()
}

func (list *SafeList[T]) Size() int {
	list.RLock()
	defer list.RUnlock()
	return list.list.Size()
}

func (list *SafeList[T]) Clear() {
	list.Lock()
	defer list.Unlock()
	list.list.Clear()
}

func (list *SafeList[T]) Sort(comparator Comparator[T]) {
	list.Lock()
	defer list.Unlock()
	list.list.Sort(comparator)
}

func (list *SafeList[T]) Swap(i, j int) {
	list.Lock()
	defer list.Unlock()
	list.list.Swap(i, j)
}

func (list *SafeList[T]) Insert(index int, values ...T) {
	list.Lock()
	defer list.Unlock()
	list.list.Insert(index, values...)
}

func (list *SafeList[T]) Set(index int, value T) {
	list.Lock()
	defer list.Unlock()
	list.list.Set(index, value)
}

func (list *SafeList[T]) String() string {
	list.RLock()
	defer list.RUnlock()
	return list.list.String()
}
