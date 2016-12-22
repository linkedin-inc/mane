package template

import (
	"sync"

	c "github.com/linkedin-inc/mane/callback"
	"github.com/linkedin-inc/mane/logger"
	"github.com/linkedin-inc/mane/middleware"
)

//短信模版名称
type Name string

const (
	//define template name constant for reference
	//NameFoo Name = "foo"
	BlankName = Name("blank")
)

//短信类别名称, 它决定了投递渠道(营销渠道/产品渠道)
type Category string

//短信通道
type Channel int

const (
	UnknownChannel Channel = iota
	MarketingChannel
	ProductionChannel
	InternalChannel
	InternationalChannel
)

func (ch Channel) String() string {
	switch ch {
	case MarketingChannel:
		return "marketing"
	case ProductionChannel:
		return "production"
	case InternalChannel:
		return "internal"
	case InternationalChannel:
		return "international"
	default:
		return "unknown"
	}
}

type SMSCategory struct {
	Name        Category `bson:"category" json:"category"`
	Channel     Channel  `bson:"channel" json:"channel"`
	Timestamp   int64    `bson:"timestamp" json:"timestamp"`
	Description string   `bson:"description" json:"description"`
	Callback    c.Name   `bson:"callback" json:"callback"`
}

type SMSTemplate struct {
	Name             Name                      `bson:"name" json:"name"`
	Category         Category                  `bson:"category" json:"category"`
	Content          string                    `bson:"content" json:"content"`
	Timestamp        int64                     `bson:"timestamp" json:"timestamp"`
	Enabled          bool                      `bson:"enabled" json:"enabled"`
	Description      string                    `bson:"description" json:"description"`
	Callback         c.Name                    `bson:"callback" json:"callback"`
	ActionStructList []middleware.ActionStruct `bson:"actions" json:"actions"`
	ActionList       []middleware.Action       `bson:"-" json:"-"`
}

var ActionCenter = make(map[string]middleware.Action)
var locker = new(sync.RWMutex)

func RegisterAction(actionName string, action middleware.Action) {
	locker.Lock()
	defer locker.Unlock()
	if _, ok := ActionCenter[actionName]; ok {
		panic("sms duplicate action registered: " + actionName)
	}
	logger.I("%v registered\n", actionName)
	ActionCenter[actionName] = action
}
