package main

import (
	"fmt"
	"sync"
)

func main() {
	var ops uint64
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				ops++ // not atomic
			}
		}()
	}

	wg.Wait()
	fmt.Println("regular ops:", ops)
}

