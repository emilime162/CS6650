package main

import (
	"fmt"  // Used for printing output
	"sync" // Provides synchronization primitives like WaitGroup
)

func main() {

	// Create a map from int to int.
	// This map is SHARED by all goroutines.
	// Go maps are NOT thread-safe.
	m := make(map[int]int)

	// WaitGroup is used to wait for all goroutines to finish.
	var wg sync.WaitGroup

	// Launch 50 goroutines
	for g := 0; g < 50; g++ {

		// Increment WaitGroup counter:
		wg.Add(1)

		// Start a goroutine.
		// IMPORTANT: we pass g as an argument so each goroutine
		// gets its own copy of g (avoids closure capture bug).
		go func(g int) {

			// Signal completion of this goroutine when it exits
			defer wg.Done()

			// Each goroutine writes 1000 key-value pairs
			for i := 0; i < 1000; i++ {

				// Write to the shared map.
				// Key:  g*1000 + i   (unique per goroutine)
				// Value: i
				//
				// PROBLEM:
				// Multiple goroutines write to the SAME map concurrently.
				// This causes a data race and can lead to:
				//   - Incorrect map contents
				//   - Program crash: "fatal error: concurrent map writes"
				m[g*1000+i] = i
			}
		}(g) // Pass g explicitly to the goroutine
	}

	// Block until all 50 goroutines call wg.Done()
	wg.Wait()

	// Print the number of entries in the map.
	// Expected value: 50 * 1000 = 50000
	// Actual result:
	//   - Might be less than 50000
	//   - Or the program may panic due to concurrent map writes
	fmt.Println("len(m):", len(m))
}
