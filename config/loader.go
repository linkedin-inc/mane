package config

import (
	"errors"

	f "github.com/linkedin-inc/mane/filter"
	"github.com/linkedin-inc/mane/logger"
	t "github.com/linkedin-inc/mane/template"
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
	EventQueue      = "sms_event_"
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
	logger.I("loaded template: %v\nloaded category: %v\nloaded strategy: %v\n", LoadedTemplates, LoadedCategories, LoadedStrategies)
}

type Watcher interface {
	Watch(c chan int64)
}

var watcher Watcher

func RegisterWatcher(w Watcher) {
	watcher = w
}

func watch() {
	watcher.Watch(hole)
}

func reload() {
	for {
		_ = <-hole
		load()
	}
}

type ConfigLoader interface {
	LoadCategory() []t.SMSCategory
	LoadTemplate() []t.SMSTemplate
	LoadStrategy() []f.Strategy
}

var loader ConfigLoader

func RegisterLoader(configLoader ConfigLoader) {
	loader = configLoader
}

func loadCategory() {
	LoadedCategories = make(map[t.Category]t.SMSCategory)
	categories := loader.LoadCategory()
	if len(categories) == 0 {
		logger.E("loaded category: %v, it seems empty, are you sure?", categories)
		return
	}
	for _, category := range categories {
		LoadedChannels[category.Name] = category.Channel
		LoadedCategories[category.Name] = category
	}
}

func loadTemplate() {
	LoadedTemplates = make(map[t.Name]t.SMSTemplate)
	templates := loader.LoadTemplate()
	if len(templates) == 0 {
		logger.E("loaded template: %v, it seems empty, are you sure?", templates)
		return
	}
	for _, template := range templates {
		LoadedTemplates[template.Name] = template
	}
}

func loadStrategy() {
	LoadedStrategies = make(map[f.Type][]f.Strategy)
	strategies := loader.LoadStrategy()
	if len(strategies) == 0 {
		logger.E("loaded strategy: %v, it seems empty, are you sure?", strategies)
		return
	}
	for _, strategy := range strategies {
		//only return enabled strategy
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
	//only return enabled template
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
