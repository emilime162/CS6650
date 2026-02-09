package main

import (
	"fmt"
	"sync"
)

func main() {
	var ops uint64
	// WaitGroup is used to wait for all goroutines to finish.
	// It keeps an internal counter:
	//   Add(n)   -> increase counter by n
	//   Done()   -> decrease counter by 1
	//   Wait()   -> block until counter reaches 0
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done() # Decrements WaitGroup counter by 1
			for j := 0; j < 1000; j++ {
				ops++ // not atomic
			}
		}()
	}
	// Block the main goroutine until:
	// all 50 goroutines have called wg.Done()
	wg.Wait()
	fmt.Println("regular ops:", ops)
}

