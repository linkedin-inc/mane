package config

import (
	"errors"
	"linkedin/log"
	"linkedin/service/mongodb"
	"linkedin/service/myredis"
	"time"

	f "github.com/linkedin-inc/mane/filter"
	t "github.com/linkedin-inc/mane/template"
	"gopkg.in/mgo.v2"
)

type SMSConfig struct {
	Username  string
	Password  string
	Endpoints []string
}

var (
	ErrTemplateNotFound     = errors.New("template not found")
	ErrCategoryNotFound     = errors.New("category not found")
	ErrStrategyNotFound     = errors.New("strategy not found")
	ErrTemplateNotAvailable = errors.New("template not available")
	ErrLoadTemplateFailed   = errors.New("failed to load template")
	ErrLoadCategoryFailed   = errors.New("failed to load category")
	ErrLoadStrategyFailed   = errors.New("failed to load strategy")

	//短信类别
	LoadedCategories = make(map[t.Category]t.SMSCategory)
	//短信通道
	LoadedChannels = make(map[t.Category]t.Channel)
	//短信模版
	LoadedTemplates = make(map[t.Name]t.SMSTemplate)
	//短信策略
	LoadedStrategies = make(map[f.Type][]f.Strategy)

	hole = make(chan int64, 1)
)

const (
	CollSMSTemplate = "sms_template"
	CollSMSCategory = "sms_category"
	CollSMSStrategy = "sms_strategy"
	EventQueue      = "sms_q"
)

//Load configuration
func Init() {
	load()
	go watch()
	go reload()
}

func load() {
	loadTemplate()
	loadCategory()
	loadStrategy()
	log.Info.Printf("loaded template: %v\nloaded category: %v\nloaded strategy: %v\n", LoadedTemplates, LoadedCategories, LoadedStrategies)
}

func watch() {
	redis := myredis.DefaultClient()
	for {
		result, err := redis.BLPop(time.Duration(0), EventQueue).Result()
		if err != nil || len(result) != 2 || result[1] != EventQueue {
			//something wrong and watch again!
			continue
		}
		hole <- time.Now().UnixNano()
	}
}

func reload() {
	for {
		_ = <-hole
		load()
	}
}

func loadCategory() {
	var categories []t.SMSCategory
	existed := mongodb.Exec(CollSMSCategory, func(c *mgo.Collection) error {
		return c.Find(nil).Sort("-timestamp").All(&categories)
	})
	if !existed || len(categories) == 0 {
		panic(ErrLoadCategoryFailed)
	}
	for _, category := range categories {
		LoadedChannels[category.Name] = category.Channel
		LoadedCategories[category.Name] = category
	}
}

func loadTemplate() {
	var templates []t.SMSTemplate
	existed := mongodb.Exec(CollSMSTemplate, func(c *mgo.Collection) error {
		return c.Find(nil).Sort("-timestamp").All(&templates)
	})
	if !existed || len(templates) == 0 {
		panic(ErrLoadTemplateFailed)
	}
	for _, template := range templates {
		LoadedTemplates[template.Name] = template
	}
}

func loadStrategy() {
	var strategies []f.Strategy
	existed := mongodb.Exec(CollSMSStrategy, func(c *mgo.Collection) error {
		return c.Find(nil).All(&strategies)
	})
	if !existed || len(strategies) == 0 {
		panic(ErrLoadStrategyFailed)
	}
	for _, strategy := range strategies {
		//FIXME only return enabled strategy?
		if !strategy.Enabled {
			continue
		}
		existing, ok := LoadedStrategies[strategy.Type]
		if !ok {
			LoadedStrategies[strategy.Type] = []f.Strategy{strategy}
		} else {
			LoadedStrategies[strategy.Type] = append(existing, strategy)
		}
	}
	f.Apply(LoadedStrategies)
}

//WhichChannel returns a channel for given category
func WhichChannel(name t.Category) (t.Channel, error) {
	channel, existed := LoadedChannels[name]
	if !existed {
		return t.UnknownChannel, ErrCategoryNotFound
	}
	return channel, nil
}

//WhichTemplate returns the enabled template for given name
func WhichTemplate(name t.Name) (*t.SMSTemplate, error) {
	smsTemplate, existed := LoadedTemplates[name]
	if !existed {
		return nil, ErrTemplateNotFound
	}
	//FIXME only return enabled template?
	if !smsTemplate.Enabled {
		return nil, ErrTemplateNotAvailable
	}
	return &smsTemplate, nil
}

func WhichCategory(name t.Category) (*t.SMSCategory, error) {
	smsCategory, existed := LoadedCategories[name]
	if !existed {
		return nil, ErrCategoryNotFound
	}
	return &smsCategory, nil
}

//StrategyFor returns all strategies for given type
func StrategyFor(typee f.Type) ([]f.Strategy, error) {
	var strategies []f.Strategy
	strategies, existed := LoadedStrategies[typee]
	if !existed {
		return strategies, ErrStrategyNotFound
	}
	return strategies, nil
}
