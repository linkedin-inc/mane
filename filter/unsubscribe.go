package filter

import t "github.com/linkedin-inc/mane/template"

type UnsubscribeFilter struct {
	Type Type `bson:"type" json:"type"`
}

func NewUnsubscribeFilter() *UnsubscribeFilter {
	return &UnsubscribeFilter{
		Type: FilterTypeUnsubscribe,
	}
}

type UnsubscribeChecker interface {
	Exists(key string) bool
}

var checker UnsubscribeChecker

func RegisterUnsubscribeChecker(c UnsubscribeChecker) {
	checker = c
}

func (f *UnsubscribeFilter) Allow(phone string, template t.Name) bool {
	return !checker.Exists(phone)
}

func (f *UnsubscribeFilter) WhichType() Type {
	return f.Type
}

func (f *UnsubscribeFilter) Apply(strategies []Strategy) {
	//do nothing
}

func (f *UnsubscribeFilter) Resolve(expression string) (interface{}, error) {
	//do nothing
	return nil, nil
}
