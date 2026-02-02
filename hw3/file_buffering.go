package main

import (
	"bufio"
	"fmt"
	"os"
	"time"
)

const (
	N        = 100000
	Filename = "output.txt"
)

func unbufferedWrite() (time.Duration, error) {
	f, err := os.Create(Filename) // create/truncate file
	if err != nil {
		return 0, err
	}
	defer f.Close()

	start := time.Now()

	for i := 0; i < N; i++ {
		// One write per iteration (unbuffered)
		// Adding a newline to make it "one line"
		line := fmt.Sprintf("line %d\n", i)

		_, err := f.Write([]byte(line))
		if err != nil {
			return 0, err
		}
	}

	return time.Since(start), nil
}

func bufferedWrite() (time.Duration, error) {
	f, err := os.Create(Filename) // create/truncate file again
	if err != nil {
		return 0, err
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	start := time.Now()

	for i := 0; i < N; i++ {
		// Write into user-space buffer
		_, err := w.WriteString(fmt.Sprintf("line %d\n", i))
		if err != nil {
			return 0, err
		}
	}

	// IMPORTANT: flush buffered data to the file
	if err := w.Flush(); err != nil {
		return 0, err
	}

	return time.Since(start), nil
}

func main() {
	d1, err := unbufferedWrite()
	if err != nil {
		fmt.Println("unbuffered error:", err)
		return
	}

	d2, err := bufferedWrite()
	if err != nil {
		fmt.Println("buffered error:", err)
		return
	}

	fmt.Println("unbuffered:", d1)
	fmt.Println("buffered:  ", d2)
}
