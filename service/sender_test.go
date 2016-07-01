package service

import "testing"

func TestGenerateSeqIDList(t *testing.T) {
	length := 1000
	N := 100
	c := make(chan int, N)
	mm := make([]map[string]struct{}, N)
	for i := 0; i < N; i++ {
		mm[i] = make(map[string]struct{})
		go func(m map[string]struct{}) {
			for _, l := range generateSeqIDList(length) {
				if _, ok := m[l]; !ok {
					m[l] = struct{}{}
				}
			}
			c <- 1
		}(mm[i])
	}
	for i := 0; i < N; i++ {
		<-c
	}
	for i := 0; i < N; i++ {
		if len(mm[i]) != length {
			t.Error("Expected %d, got %d", length, len(mm[i]))
		}
	}
}
