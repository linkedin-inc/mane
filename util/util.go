package util

import (
	"strconv"

	"github.com/linkedin-inc/mane/logger"
)

func Atoi64Safe(s string, x int64) int64 {
	if s == "" {
		return 0
	}
	i, err := strconv.ParseInt(s, 10, 0)
	if err != nil {
		logger.E("[panic] parse atoi64 s:%s, err:%v\n", s, err)
		return x
	}
	return i
}

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

// Atoi32 convert string to i32 without panic
func Atoi32Safe(s string, defaultVal int32) int32 {
	if s == "" {
		return defaultVal
	}

	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		logger.E("[panic] parse atoi32 s:%s, err:%v\n", s, err)
		return defaultVal
	}
	return int32(i)
}

func Itoa(i int) string {
	return strconv.FormatInt(int64(i), 10)
}

func IsProduction() bool {
	return true
}
