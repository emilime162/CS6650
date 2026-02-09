package main // Entry package for an executable Go program

import (
	"bufio" // Provides buffered I/O (user-space buffering)
	"fmt"   // String formatting and printing
	"os"    // OS-level functions (files, syscalls)
	"time"  // Time measurement
)

const (
	N        = 100000
	Filename = "output.txt"
)

//
// UNBUFFERED VERSION
// Each write goes directly to the OS (many syscalls)
//
func unbufferedWrite() (time.Duration, error) {

	// Ask the OS to create (or truncate) the file
	// Returns a file descriptor handled by the OS kernel
	f, err := os.Create(Filename)
	if err != nil {
		return 0, err
	}

	// Ensure file is closed when function returns
	// Releases OS resources
	defer f.Close()

	// Record start time for benchmarking
	start := time.Now()

	// Loop N times
	for i := 0; i < N; i++ {

		// Create a string for one line
		// This allocates memory every iteration
		line := fmt.Sprintf("line %d\n", i)

		// Write directly to the file
		// Each call likely triggers a syscall (user → kernel)
		_, err := f.Write([]byte(line))
		if err != nil {
			return 0, err
		}
	}

	// Return how long all writes took
	return time.Since(start), nil
}

//
// BUFFERED VERSION
// Writes first go to user-space memory, then flushed in batches
//
func bufferedWrite() (time.Duration, error) {

	// Create or truncate the file again
	f, err := os.Create(Filename)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// Wrap the file with a buffered writer
	// Data is stored in memory first, not sent to OS immediately
	w := bufio.NewWriter(f)

	// Record start time
	start := time.Now()

	// Loop N times
	for i := 0; i < N; i++ {

		// Format one line
		// Still allocates a string each iteration
		line := fmt.Sprintf("line %d\n", i)

		// Write into the in-memory buffer
		// Usually just a memory copy (fast)
		// Does NOT immediately trigger an OS write
		_, err := w.WriteString(line)
		if err != nil {
			return 0, err
		}
	}

	// IMPORTANT:
	// Flush forces buffered data to be written to the file
	// This is where actual OS writes (syscalls) happen
	if err := w.Flush(); err != nil {
		return 0, err
	}

	// Return elapsed time
	return time.Since(start), nil
}

func main() {

	// Run unbuffered version
	d1, err := unbufferedWrite()
	if err != nil {
		fmt.Println("unbuffered error:", err)
		return
	}

	// Run buffered version
	d2, err := bufferedWrite()
	if err != nil {
		fmt.Println("buffered error:", err)
		return
	}

	// Print timing results
	fmt.Println("unbuffered:", d1)
	fmt.Println("buffered:  ", d2)
}
