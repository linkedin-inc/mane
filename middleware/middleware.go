package middleware

import (
	"github.com/linkedin-inc/mane/logger"
	"github.com/linkedin-inc/mane/model"
)

type Action interface {
	Name() string
	Call(context model.SMSContext, next func() bool) bool
}

type Middleware struct {
	actions []Action
}

func (m *Middleware) Append(action Action) {
	m.actions = append(m.actions, action)
}

func (m *Middleware) Prepend(action Action) {
	actions := make([]Action, len(m.actions)+1)
	actions[0] = action
	copy(actions[1:], m.actions)
	m.actions = actions
}

func (m *Middleware) Call(contexts []model.SMSContext) []model.SMSContext {
	var allowedContexts []model.SMSContext
	for _, context := range contexts {
		continuation(m.actions, context, func() {
			allowedContexts = append(allowedContexts, context)
		})()
	}
	return allowedContexts
}

func continuation(actions []Action, context model.SMSContext, final func()) func() bool {
	return func() (acknowledge bool) {
		if len(actions) > 0 {
			acknowledge = actions[0].Call(context, continuation(actions[1:], context, final))
			if !acknowledge {
				logger.I("%v prevented by %s\n", context, actions[0].Name())
				return
			}
		} else {
			final()
		}
		return true
	}
}

func NewMiddleware(actions ...Action) *Middleware {
	middleware := &Middleware{}
	for _, a := range actions {
		middleware.Append(a)
	}
	return middleware
}
