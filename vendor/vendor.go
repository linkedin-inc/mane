package vendor

import (
	"errors"
	"fmt"

	"github.com/linkedin-inc/mane/model"
	t "github.com/linkedin-inc/mane/template"
)

var (
	ErrNotInProduction    = errors.New("not in production")
	ErrSendSMSFailed      = errors.New("send sms failed")
	ErrGetStatusFailed    = errors.New("get status failed")
	ErrGetReplyFailed     = errors.New("get reply failed")
	ErrQueryBalanceFailed = errors.New("query balance failed")
	ErrVendorNotFound     = errors.New("vendor not found")
)

var Registry struct {
	Channels map[t.Channel][]Vendor
	Vendors  map[Name][]Vendor
}

func init() {
	prepare()
	fmt.Println("prepared vendors:", Registry)
}

func prepare() {
	//montnets
	montnetsForProduction := NewMontnets("", "", "", "", "")
	montnetsForProduction.Register(t.ProductionChannel)
	montnetsForMarketing := NewMontnets("", "", "", "", "")
	montnetsForMarketing.Register(t.MarketingChannel)
	//yunpian
}

type Name string

//Vendor represents a SMS vendor, it can preforms two behaviors, send sms and check delivery status and pull reply.
type Vendor interface {
	Register(channel t.Channel)
	Name() Name
	Send(seqID string, phoneArray []string, contentArray []string) error
	Status() ([]model.DeliveryStatus, error)
	Reply() ([]model.Reply, error)
	GetBalance() (string, error)
}

//GetByChannel return a registered SMS vendor for given channel
func GetByChannel(channel t.Channel) (Vendor, error) {
	vendors, existed := Registry.Channels[channel]
	if !existed || len(vendors) == 0 {
		return nil, ErrVendorNotFound
	}
	return choose(vendors)
}

func choose(vendors []Vendor) (Vendor, error) {
	//TODO choose random or according to strategy, now we just pick the 1st one.
	return vendors[0], nil
}

//GetByName return a vendor for given name
func GetByName(name Name) ([]Vendor, error) {
	vendors, existed := Registry.Vendors[name]
	if !existed || len(vendors) == 0 {
		return nil, ErrVendorNotFound
	}
	return vendors, nil
}
