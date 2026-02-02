package main

// fmt: used for printing output
// sync: provides concurrency primitives (Mutex, WaitGroup)
// time: used to measure execution time
import (
	"fmt"
	"sync"
	"time"
)

// SafeMap wraps a regular Go map together with a mutex.
// The mutex ensures that concurrent access to the map is safe.
type SafeMap struct {
	mu sync.Mutex   // Mutex to protect access to the map
	m  map[int]int  // The underlying map (not thread-safe by itself)
}

// NewSafeMap initializes and returns a pointer to a SafeMap.
// This ensures the internal map is properly created before use.
func NewSafeMap() *SafeMap {
	return &SafeMap{
		m: make(map[int]int),
	}
}

// Set writes a key/value pair into the map.
// The mutex is locked before writing and unlocked afterward
// to prevent concurrent map writes.
func (s *SafeMap) Set(k, v int) {
	s.mu.Lock()      // Lock before entering critical section
	s.m[k] = v       // Write to the shared map
	s.mu.Unlock()    // Unlock after write is complete
}

// Len safely returns the number of elements in the map.
// Reading len(m) also requires a lock because the map
// may be modified concurrently by other goroutines.
func (s *SafeMap) Len() int {
	s.mu.Lock()        // Lock before reading shared state
	n := len(s.m)      // Read map length
	s.mu.Unlock()      // Unlock after read
	return n
}

func main() {
	// Create a new thread-safe map
	sm := NewSafeMap()

	// WaitGroup is used to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Record start time
	start := time.Now()

	// Spawn 50 goroutines
	for g := 0; g < 50; g++ {
		wg.Add(1)

		// Each goroutine writes 1000 distinct key/value pairs
		go func(g int) {
			defer wg.Done() // Signal completion when goroutine exits

			for i := 0; i < 1000; i++ {
				// Each key is unique: g*1000 + i
				sm.Set(g*1000+i, i)
			}
		}(g)
	}

	// Wait until all goroutines have finished
	wg.Wait()

	// Measure total execution time
	elapsed := time.Since(start)

	// Print final map size and total time taken
	fmt.Println("len(m):", sm.Len())
	fmt.Println("time:", elapsed)
}
