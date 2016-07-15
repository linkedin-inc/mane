package filter

import (
	"encoding/json"
	"linkedin/log"
	"linkedin/service/myredis"
	"linkedin/util"
	"strconv"
	"time"

	"github.com/go-errors/errors"
	t "github.com/linkedin-inc/mane/template"
)

const (
	Check = `
	local count = tonumber(redis.call("INCR", KEYS[1]))
	if count == 1 then
		redis.call("EXPIRE", KEYS[1], tonumber(ARGV[1]))
	end
	return count
	`
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
		log.Error.Printf("failed to calculate expiration due to invalid time unit")
		return false
	}
	if !util.IsProduction() {
		//ignore actual setting in non-production environment and use 5 mins as default expiration
		expiration = int64(5 * time.Minute / time.Second)
	}
	redisClient := myredis.DefaultClient()
	res, err := redisClient.Eval(Check, []string{"cnt_" + phone + "_" + string(template)}, []string{strconv.FormatInt(expiration, 10)}).Result()
	if err != nil {
		//FIXME should allow or discard when check failed?
		log.Error.Printf("occur error when check allowed: %v\n", err)
		return false
	}
	return res.(int64) <= int64(strategy.Count)
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
			log.Error.Printf("occur error when resolve strategy[%v] expression[%v]: %v\n", strategy.Type, strategy.Expression, err)
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
