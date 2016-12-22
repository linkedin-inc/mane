package mane

import (
	"github.com/linkedin-inc/mane/config"
	"github.com/linkedin-inc/mane/template"
	"github.com/linkedin-inc/mane/vendors"
)

func InitSMS(conf map[template.Channel]config.SMSConfig) {
	config.Init()
	vendors.Prepare(conf)
}
