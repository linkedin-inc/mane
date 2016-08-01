package model

import "time"

const (
	CollSMSHistory   = "sms_history"
	CollSMStatus     = "sms_status"
	CollSMSReply     = "sms_reply"
	CollUnsubscriber = "sms_unsubscriber"
)

type SMSState int

const (
	SMSStateChecked = iota + 1
	SMSStateUnchecked
	SMSStateProcessed
	SMSStateUnprocessed
	SMSStateFailed
)

type SMSHistory struct {
	MsgID     int64     `bson:"msg_id" json:"msg_id"`
	Timestamp time.Time `bson:"timestamp" json:"timestamp"`
	Phone     string    `bson:"phone" json:"phone"`
	Content   string    `bson:"content" json:"content"`
	Template  string    `bson:"template" json:"template"`
	Category  string    `bson:"category" json:"category"`
	Channel   int       `bson:"channel" json:"channel"`
	Vendor    string    `bson:"vendor" json:"vendor"`
	State     SMSState  `bson:"state" json:"state"`
}

type DeliveryStatus struct {
	MsgID      int64     `bson:"msg_id" json:"msg_id"`
	Timestamp  time.Time `bson:"timestamp" json:"timestamp"`
	Phone      string    `bson:"phone" json:"phone"`
	StatusCode int32     `bson:"status_code" json:"status_code"`
	ErrorMsg   string    `bson:"error_msg" json:"error_msg"`
}

type Reply struct {
	Timestamp time.Time `bson:"timestamp" json:"timestamp"`
	Phone     string    `bson:"phone" json:"phone"`
	Msg       string    `bson:"msg" json:"msg"`
}

type Unsubscriber struct {
	Timestamp time.Time `bson:"timestamp" json:"timestamp"`
	Phone     string    `bson:"phone" json:"phone"`
}

type SMSContext struct {
	Phone     string            `json:"phone"`
	Template  string            `json:"template"`
	Variables map[string]string `json:"variables"`
}

func NewSMSContext(phone string, template string, variables map[string]string) *SMSContext {
	return &SMSContext{
		Phone:     phone,
		Template:  template,
		Variables: variables,
	}
}
