package util

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestConcurrentMap(t *testing.T) {
	fmt.Println(time.Now())
	rand.Seed(time.Now().UnixNano())
	m := NewConcurrentMap()
	N := 1000
	c := make(chan int, N)
	for i := 0; i < N; i++ {
		go func() {
			m.Set("aaa", RandStringRunes(10))
			c <- 1
		}()
		go func() {
			m.Get("aaa")
			c <- 1
		}()

	}
	for i := 0; i < N*2; i++ {
		<-c
	}
	fmt.Println(time.Now())
}

func TestNormalMap(t *testing.T) {
	fmt.Println(time.Now())
	rand.Seed(time.Now().UnixNano())
	m := make(map[string]string)
	N := 1000
	c := make(chan int, N)
	for i := 0; i < N; i++ {
		go func() {
			m["aaa"] = RandStringRunes(10)
			c <- 1
		}()
		go func() {
			_ = m["aaa"]
			c <- 1
		}()

	}
	for i := 0; i < N*2; i++ {
		<-c
	}
	fmt.Println(time.Now())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
