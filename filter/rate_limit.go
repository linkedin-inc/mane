package filter

import (
	"encoding/json"
	"time"

	"github.com/go-errors/errors"
	"github.com/linkedin-inc/mane/logger"
	t "github.com/linkedin-inc/mane/template"
)

var ErrResolveFailed = errors.New("failed to resolve expression")

//for further extention, RateLimitStrategy can hold bare script
type RateLimitStrategy struct {
	Duration time.Duration `json:"duration"`
	Unit     string        `json:"unit"`
	Count    uint32        `json:"count"`
}

type RateLimitFilter struct {
	Type       Type                         `bson:"type" json:"type"`
	Strategies map[t.Name]RateLimitStrategy `bson:"strategies" json:"strategies"`
}

func NewRateLimitFilter() *RateLimitFilter {
	return &RateLimitFilter{
		Type:       FilterTypeRateLimit,
		Strategies: make(map[t.Name]RateLimitStrategy),
	}
}

type RateLimitChecker interface {
	IsExceeded(key string, expiration, threshold int64) bool
}

var ratelimitChecker RateLimitChecker

func RegisterRateLimitChecker(c RateLimitChecker) {
	ratelimitChecker = c
}

func (f *RateLimitFilter) Allow(phone string, template t.Name) bool {
	strategy, existed := f.Strategies[template]
	if !existed {
		return true
	}
	var expiration int64
	switch strategy.Unit {
	case "s", "S":
		expiration = int64(strategy.Duration * time.Second / time.Second)
		break
	case "m", "M":
		expiration = int64(strategy.Duration * time.Minute / time.Second)
		break
	case "h", "H":
		expiration = int64(strategy.Duration * time.Hour / time.Second)
		break
	case "d", "D":
		expiration = int64(strategy.Duration * time.Hour * 24 / time.Second)
		break
	default:
		logger.E("failed to calculate expiration due to invalid time unit")
		return false
	}
	key := "cnt_" + phone + "_" + string(template)
	if ratelimitChecker.IsExceeded(key, expiration, int64(strategy.Count)) {
		return true
	}
	logger.I("[sms] phone:%s template:%v prevented by RateLimitFilter, key:%s, strategy.Count:%d, expiration:%d", phone, template, key, int64(strategy.Count), expiration)
	return false
}

func (f *RateLimitFilter) WhichType() Type {
	return f.Type
}

func (f *RateLimitFilter) Apply(strategies []Strategy) {
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
		f.Strategies[strategy.Template] = resolved.(RateLimitStrategy)
	}
}

func (f *RateLimitFilter) Resolve(expression string) (interface{}, error) {
	//resolve expression to RateLimitExpression
	var exp RateLimitStrategy
	err := json.Unmarshal([]byte(expression), &exp)
	if err != nil {
		return nil, ErrResolveFailed
	}
	return exp, nil
}
