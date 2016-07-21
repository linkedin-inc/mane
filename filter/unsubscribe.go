package filter

import (
	"encoding/json"

	"github.com/linkedin-inc/mane/logger"
	t "github.com/linkedin-inc/mane/template"
)

type UnsubscribeStrategy struct {
	Ignore bool `json:"ignore"`
}

type UnsubscribeFilter struct {
	Type       Type            `bson:"type" json:"type"`
	Strategies map[t.Name]bool `bson:"strategies" json:"strategies"`
}

func NewUnsubscribeFilter() *UnsubscribeFilter {
	return &UnsubscribeFilter{
		Type:       FilterTypeUnsubscribe,
		Strategies: make(map[t.Name]bool),
	}
}

type UnsubscribeChecker interface {
	Exists(key string) bool
}

var unsubscribechecker UnsubscribeChecker

func RegisterUnsubscribeChecker(c UnsubscribeChecker) {
	unsubscribechecker = c
}

func (f *UnsubscribeFilter) Allow(phone string, template t.Name) bool {
	ignored, existed := f.Strategies[template]
	if existed && ignored {
		return true
	}
	if unsubscribechecker.Exists(phone) {
		logger.I("[sms] phone:%s template:%v prevented by UnsubscribeFilter", phone, template)
		return false
	}
	return true
}

func (f *UnsubscribeFilter) WhichType() Type {
	return f.Type
}

func (f *UnsubscribeFilter) Apply(strategies []Strategy) {
	if len(strategies) == 0 {
		return
	}
	for _, strategy := range strategies {
		resolved, err := f.Resolve(strategy.Expression)
		if err != nil {
			//FIXME discard when resolve failed?
			logger.E("occur error when resolve strategy[%v] expression[%v]: %v\n", strategy.Type, strategy.Expression, err)
			continue
		}
		f.Strategies[strategy.Template] = resolved.(UnsubscribeStrategy).Ignore
	}
}

func (f *UnsubscribeFilter) Resolve(expression string) (interface{}, error) {
	//resolve expression to UnsubscribeStratege
	var resolved UnsubscribeStrategy
	err := json.Unmarshal([]byte(expression), &resolved)
	if err != nil {
		return nil, ErrResolveFailed
	}
	return resolved, nil
}
