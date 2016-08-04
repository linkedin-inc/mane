package middleware

import (
	"github.com/linkedin-inc/mane/logger"
	m "github.com/linkedin-inc/mane/model"
)

type ErrorReport struct{}

func NewErrorReport() *ErrorReport {
	return &ErrorReport{}
}

func (*ErrorReport) Name() string {
	return "ErrorReport"
}

func (*ErrorReport) Call(context m.SMSContext, next func() bool) (acknowledge bool) {
	defer func() {
		if err := recover(); err != nil {
			logger.E("sms middleware error report: %v\n", err)
		}
	}()
	next()
	return true
}
