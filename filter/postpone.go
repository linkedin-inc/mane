package filter

import (
	"encoding/json"
	"time"

	"github.com/linkedin-inc/go-workers"
	"github.com/linkedin-inc/mane/constant"
	"github.com/linkedin-inc/mane/logger"
	m "github.com/linkedin-inc/mane/model"
	t "github.com/linkedin-inc/mane/template"
)

const (
	DateLayout        = "2006-01-02"
	TimeLayout        = "15:04:05"
	DateAndTimeLayout = "2006-01-02 15:04:05"
)

type PostponeStrategy struct {
	Begin time.Time
	End   time.Time
}

type Exp struct {
	Begin string `json:"begin"`
	End   string `json:"end"`
}

type PostponeFilter struct {
	Type       Type                        `bson:"type" json:"type"`
	Strategies map[t.Name]PostponeStrategy `bson:"strategies" json:"strategies"`
}

func NewPostponeFilter() *PostponeFilter {
	return &PostponeFilter{
		Type:       FilterTypePostpone,
		Strategies: make(map[t.Name]PostponeStrategy),
	}
}

func (f *PostponeFilter) Allow(phone string, template t.Name) bool {
	strategy, existed := f.Strategies[template]
	if !existed {
		return true
	}
	now := time.Now()
	if now.After(strategy.Begin) && now.Before(strategy.End) {
		return true
	}
	if now.Before(strategy.Begin) {
		postpone(strategy.Begin, phone, template)
	} else if now.After(strategy.End) {
		postpone(strategy.Begin.Add(time.Hour*24), phone, template)
	}
	return false
}

func postpone(when time.Time, phone string, template t.Name) {
	v, existed := FindVariables(phone, template)
	if !existed {
		//inconsistent, discard!
		return
	}
	job := m.SMSJob{
		Phone:     phone,
		Template:  string(template),
		Variables: v,
	}
	_, err := workers.EnqueueAt(constant.PostponeQueue, "", when, job)
	if err != nil {
		logger.E("occur error when write queue to postpone: %v\n", err)
	}
}

func (f *PostponeFilter) WhichType() Type {
	return f.Type
}
func (f *PostponeFilter) Apply(strategies []Strategy) {
	if len(strategies) == 0 {
		return
	}
	for _, s := range strategies {
		resolved, err := f.Resolve(s.Expression)
		if err != nil {
			//FIXME discard when resolve failed?
			logger.E("occur error when resolve strategy[%v] expression[%v]: %v\n", s.Type, s.Expression, err)
			continue
		}
		f.Strategies[s.Template] = resolved.(PostponeStrategy)
	}
}

func (f *PostponeFilter) Resolve(expression string) (interface{}, error) {
	var exp Exp
	err := json.Unmarshal([]byte(expression), &exp)
	if err != nil {
		return nil, ErrResolveFailed
	}
	prefix := time.Now().Format(DateLayout)
	t1, err := time.ParseInLocation(DateAndTimeLayout, prefix+" "+exp.Begin, time.Local)
	if err != nil {
		return nil, ErrResolveFailed
	}
	t2, err := time.ParseInLocation(DateAndTimeLayout, prefix+" "+exp.End, time.Local)
	if err != nil {
		return nil, ErrResolveFailed
	}
	return PostponeStrategy{Begin: t1, End: t2}, nil
}
