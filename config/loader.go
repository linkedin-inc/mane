package config

import (
	"errors"

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
	ErrTemplateNotAvailable = errors.New("template not available")
	ErrLoadTemplateFailed   = errors.New("failed to load template")
	ErrLoadCategoryFailed   = errors.New("failed to load category")

	//短信类别
	LoadedCategories = make(map[t.Category]t.SMSCategory)
	//短信通道
	LoadedChannels = make(map[t.Category]t.Channel)
	//短信模版
	LoadedTemplates = make(map[t.Name]t.SMSTemplate)

	hole = make(chan int64, 1)
)

const (
	CollSMSTemplate = "sms_template"
	CollSMSCategory = "sms_category"
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
	logger.I("loaded template: %v\nloaded category: %v\n", LoadedTemplates, LoadedCategories)
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
