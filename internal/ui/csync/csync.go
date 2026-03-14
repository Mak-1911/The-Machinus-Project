// Package csync provides concurrent-safe data structures.
package csync

import "sync"

// Map is a concurrent-safe map.
type Map[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]V
}

// NewMap creates a new concurrent-safe map.
func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{
		data: make(map[K]V),
	}
}

// Load returns the value stored in the map for a key, or zero if no value is present.
// The ok result indicates whether a value was found in the map.
func (m *Map[K, V]) Load(key K) (value V, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok = m.data[key]
	return
}

// Get is an alias for Load for compatibility.
func (m *Map[K, V]) Get(key K) (V, bool) {
	return m.Load(key)
}

// Store sets the value for a key.
func (m *Map[K, V]) Store(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
}

// Set is an alias for Store for compatibility.
func (m *Map[K, V]) Set(key K, value V) {
	m.Store(key, value)
}

// LoadAndStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (m *Map[K, V]) LoadAndStore(key K, value V) (actual V, loaded bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	actual, loaded = m.data[key]
	if loaded {
		return actual, true
	}
	m.data[key] = value
	return value, false
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (m *Map[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	actual, loaded = m.data[key]
	if loaded {
		return actual, true
	}
	m.data[key] = value
	return value, false
}

// Delete deletes the value for a key.
func (m *Map[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
func (m *Map[K, V]) Range(f func(key K, value V) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.data {
		if !f(k, v) {
			break
		}
	}
}

// Len returns the number of elements in the map.
func (m *Map[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

// Seq returns all elements in the map as a slice.
func (m *Map[K, V]) Seq() []V {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]V, 0, len(m.data))
	for _, v := range m.data {
		result = append(result, v)
	}
	return result
}

// Slice is a concurrent-safe slice.
type Slice[T any] struct {
	mu   sync.RWMutex
	data []T
}

// NewSlice creates a new concurrent-safe slice.
func NewSlice[T any]() *Slice[T] {
	return &Slice[T]{
		data: make([]T, 0),
	}
}

// NewSliceFrom creates a new concurrent-safe slice from an existing slice.
func NewSliceFrom[T any](data []T) *Slice[T] {
	return &Slice[T]{
		data: data,
	}
}

// Len returns the length of the slice.
func (s *Slice[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// Append appends values to the slice.
func (s *Slice[T]) Append(values ...T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = append(s.data, values...)
}

// Get returns the element at index i.
func (s *Slice[T]) Get(i int) (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if i < 0 || i >= len(s.data) {
		var zero T
		return zero, false
	}
	return s.data[i], true
}

// Range calls f for each element in the slice.
func (s *Slice[T]) Range(f func(i int, v T) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i, v := range s.data {
		if !f(i, v) {
			break
		}
	}
}

// Seq2 returns all elements in the slice as a sequence.
func (s *Slice[T]) Seq2() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]T, len(s.data))
	copy(result, s.data)
	return result
}
