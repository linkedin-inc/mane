package template

import (
	"sync"

	c "github.com/linkedin-inc/mane/callback"
	"github.com/linkedin-inc/mane/logger"
	"github.com/linkedin-inc/mane/middleware"
)

//短信模版名称, 需要和CRM中定义的相同, 建议在CRM中添加新模版后对应添加新的常量定义
type Name string

const (
	//define template name constant for reference
	//NameFoo Name = "foo"
	BlankName = Name("blank")
)

//短信类别名称, 它决定了短信ID前缀以及短信投递渠道(营销渠道/产品渠道), 需要和CRM中定义的相同, 建议在CRM中添加新类别后对应添加新的常量定义
type Category string

const (
	//define category constant for reference
	//CategoryBar Category = "bar"
	BlankCategory = Category("blank")
)

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
	default:
		return "unknown"
	}
}

func WhichChannel(str string) Channel {
	switch str {
	case "production":
		return ProductionChannel
	case "marketing":
		return MarketingChannel
	case "internal":
		return InternalChannel
	default:
		return UnknownChannel
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
