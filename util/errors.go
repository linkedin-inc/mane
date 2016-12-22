package util

import (
	"errors"
	"strings"
)

type ErrorGroup struct {
	Errors []error
}

func (e *ErrorGroup) Add(err error) {
	if err != nil {
		e.Errors = append(e.Errors, err)
	}
}

func (e *ErrorGroup) Finalize() error {
	if len(e.Errors) == 0 {
		return nil
	}
	if len(e.Errors) == 1 {
		return e.Errors[0]
	}
	msgs := make([]string, 0, len(e.Errors))
	for _, err := range e.Errors {
		msgs = append(msgs, err.Error())
	}
	return errors.New(strings.Join(msgs, "\n"))
}
