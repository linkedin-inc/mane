package util

import (
	"fmt"
	"testing"
)

var pool *Pool

func TestPool(t *testing.T) {
	pool = NewPool(10, 10)
	defer pool.Release()

	testCount := 100
	pool.WaitCount(testCount)
	for i := 0; i < testCount; i++ {
		count := i
		pool.JobQueue <- func() {
			defer pool.JobDone()
			fmt.Printf("hello %d\n", count)
		}
	}
	pool.WaitAll()
	fmt.Println("done")
}
