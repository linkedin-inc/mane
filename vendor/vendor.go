package vendor

import (
	"errors"

	c "github.com/linkedin-inc/mane/config"
	"github.com/linkedin-inc/mane/logger"
	m "github.com/linkedin-inc/mane/model"
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

type vendorRegistry struct {
	Channel2Vendors map[t.Channel][]Vendor
	Name2Vendors    map[Name][]Vendor
}

var registry vendorRegistry

func init() {
	registry = vendorRegistry{
		Channel2Vendors: make(map[t.Channel][]Vendor),
		Name2Vendors:    make(map[Name][]Vendor),
	}
}

func Prepare(config map[t.Channel]c.SMSConfig) {
	for k, v := range config {
		Register(k, NewMontnets(v.Username, v.Password, v.Endpoints[0], v.Endpoints[1], v.Endpoints[2], v.Endpoints[3]))
	}
	logger.I("prepared vendors:%v", registry)
}

type Name string

//Vendor represents a SMS vendor, it can preforms two behaviors, send sms and check delivery status and pull reply.
type Vendor interface {
	Name() Name
	Send(contexts []*m.SMSContext) ([]*m.SMSContext, error)
	MultiXSend(contexts []*m.SMSContext) ([]*m.SMSContext, error)
	Status() ([]*m.DeliveryStatus, error)
	Reply() ([]*m.Reply, error)
	GetBalance() (string, error)
}

func Register(ch t.Channel, v Vendor) {
	vendors, existed := registry.Channel2Vendors[ch]
	if !existed {
		registry.Channel2Vendors[ch] = []Vendor{v}
	} else {
		registry.Channel2Vendors[ch] = append(vendors, v)
	}
	vendors, existed = registry.Name2Vendors[v.Name()]
	if !existed {
		registry.Name2Vendors[v.Name()] = []Vendor{v}
	} else {
		registry.Name2Vendors[v.Name()] = append(vendors, v)
	}
}

//GetByChannel return a registered SMS vendor for given channel
func GetByChannel(channel t.Channel) (Vendor, error) {
	vendors, existed := registry.Channel2Vendors[channel]
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
	vendors, existed := registry.Name2Vendors[name]
	if !existed || len(vendors) == 0 {
		return nil, ErrVendorNotFound
	}
	return vendors, nil
}
