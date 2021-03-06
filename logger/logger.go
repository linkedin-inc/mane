package logger

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
)

var (
	dbg *log.Logger
	inf *log.Logger
	err *log.Logger
)

func init() {
	if os.Getenv("CHITU_ENV") == "production" {
		initLogger(ioutil.Discard, os.Stdout, os.Stderr)
	} else {
		initLogger(os.Stdout, os.Stdout, os.Stderr)
	}
}

func initLogger(debug, info, error io.Writer) {
	format := log.Ldate | log.Ltime
	dbg = log.New(debug, "[DEBUG]: ", format|log.Lshortfile)
	inf = log.New(info, "[INFO]: ", format)
	err = log.New(error, "[ERROR]: ", format|log.Llongfile)
}

func I(format string, args ...interface{}) {
	_ = inf.Output(2, fmt.Sprintf(format, args...))
}

func E(format string, args ...interface{}) {
	_ = err.Output(2, fmt.Sprintf(format, args...))
}

func D(format string, args ...interface{}) {
	_ = dbg.Output(2, fmt.Sprintf(format, args...))
}
