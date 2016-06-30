package mane

import (
	"fmt"
	"os"

	"github.com/linkedin-inc/mane/config"
	"github.com/linkedin-inc/mane/template"
	"github.com/linkedin-inc/mane/vendor"
)

func InitSMS(conf map[template.Channel]config.SMSConfig) {
	// check config
	if len(conf) != 4 {
		fmt.Fprintln(os.Stderr, "illegal sms config map")
		return
	}
	config.Init()
	vendor.Prepare(conf)
}

func InitPush() {
	//TODO
}
