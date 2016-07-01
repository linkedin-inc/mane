package util

import (
	"strconv"
	"time"
)

func Atoi64(s string) int64 {
	if s == "" {
		return 0
	}
	i, err := strconv.ParseInt(s, 10, 0)
	if err != nil {
		panic(err)
	}
	return i
}

func Atoi64Safe(s string, x int64) int64 {
	if s == "" {
		return 0
	}
	i, err := strconv.ParseInt(s, 10, 0)
	if err != nil {
		return x
	}
	return i
}

func CurrentTimeMillis() int64 {
	return time.Now().UnixNano() / 1e6
}