package main

import (
	"fmt"
	"runtime"
	"time"
)

const N = 1_000_000

func pingPong() (time.Duration, time.Duration) {
	ping := make(chan struct{})
	pong := make(chan struct{})
	done := make(chan struct{})

	start := time.Now()

	// Goroutine A: ping -> pong (N times)
	go func() {
		for i := 0; i < N; i++ {
			// "receive a value from the channel ping"
			<-ping
			pong <- struct{}{}
		}
	}()

	// Goroutine B: pong -> ping (N-1 times), then signal done after last receive
	go func() {
		for i := 0; i < N; i++ {
			// Receive from pong (blocks until A sends)
			<-pong
			if i == N-1 {
				close(done)
				return
			}
			// Send back to ping so A can continue

			ping <- struct{}{}
		}
	}()

	// Kick off
	ping <- struct{}{}

	// Wait until B confirms it completed all iterations
	<-done

	total := time.Since(start)
	avg := total / (2 * N) // 2 handoffs per round-trip (A->B and B->A)

	return total, avg
}

func main() {
	runtime.GOMAXPROCS(1)
	t1, avg1 := pingPong()
	fmt.Println("GOMAXPROCS=1 total:", t1, "avg per handoff:", avg1)

	runtime.GOMAXPROCS(runtime.NumCPU())
	fmt.Println("CPUs:", runtime.NumCPU())

	t2, avg2 := pingPong()
	fmt.Println("GOMAXPROCS=NumCPU total:", t2, "avg per handoff:", avg2)
}
