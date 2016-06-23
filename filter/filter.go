package filter

import (
	"errors"
	"linkedin/log"

	t "github.com/linkedin-inc/mane/template"
)

var (
	errNotAllowed = errors.New("not allowed")
)

var head *Chain

type Chain struct {
	Filter Filter
	Next   *Chain
}

func NewChain(filter Filter) *Chain {
	return &Chain{
		Filter: filter,
		Next:   nil,
	}
}

func (c *Chain) Braid(filter Filter) *Chain {
	ptr := c
	for ptr.Next != nil {
		ptr = ptr.Next
	}
	ptr.Next = NewChain(filter)
	return c
}

type Strategy struct {
	Type       Type   `bson:"type" json:"type"`
	Template   t.Name `bson:"template" json:"template"`
	Expression string `bson:"expression" json:"expression"`
	Enabled    bool   `bson:"enabled" json:"enabled"`
}

type Type int

const (
	FilterTypeUnsubscribe Type = iota + 1
	FilterTypeRateLimit
	FilterTypePostpone
)

func (t Type) String() string {
	switch t {
	case FilterTypeRateLimit:
		return "rate_limit"
	case FilterTypeUnsubscribe:
		return "unsubscribe"
	case FilterTypePostpone:
		return "postpone"
	default:
		return "unkown"
	}
}

type Filter interface {
	Allow(phone string, template t.Name) bool
	WhichType() Type
	Apply(strategies []Strategy)
	Resolve(expression string) (interface{}, error)
}

func init() {
	prepare()
}

func prepare() {
	head = NewChain(NewUnsubscribeFilter()).Braid(NewPostponeFilter()).Braid(NewRateLimitFilter())
}

var variablesHolder = make(map[string]map[string]string)

func StoreVariables(phones []string, template t.Name, variables map[string]string) {
	for _, phone := range phones {
		variablesHolder[phone+":"+string(template)] = variables
	}
}

func ClearVariables(phones []string, template t.Name) {
	for _, phone := range phones {
		delete(variablesHolder, phone+":"+string(template))
	}
}

func FindVariables(phone string, template t.Name) (map[string]string, bool) {
	v, ok := variablesHolder[phone+":"+string(template)]
	return v, ok
}

func ProcessChain(phoneArray []string, template t.Name) []string {
	var allowed []string
	var notAllowed []string
	for _, phone := range phoneArray {
		err := process(phone, template)
		if err != nil && err == errNotAllowed {
			notAllowed = append(notAllowed, phone)
			continue
		}
		allowed = append(allowed, phone)
	}
	log.Info.Printf("allowed: %v, not-allowed: %v\n", allowed, notAllowed)
	return allowed
}

func process(phone string, template t.Name) error {
	ptr := head
	for ptr != nil {
		if ptr.Filter.Allow(phone, template) {
			ptr = ptr.Next
		} else {
			return errNotAllowed
		}
	}
	return nil
}

func Apply(strategies map[Type][]Strategy) {
	ptr := head
	for ptr != nil {
		s, ok := strategies[ptr.Filter.WhichType()]
		if ok && len(s) > 0 {
			ptr.Filter.Apply(s)
		}
		ptr = ptr.Next
	}
}
