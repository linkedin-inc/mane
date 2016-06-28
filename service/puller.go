package service

import (
	"linkedin/log"
	"linkedin/service/mongodb"
	"strconv"
	"strings"

	"github.com/go-errors/errors"
	cb "github.com/linkedin-inc/mane/callback"
	c "github.com/linkedin-inc/mane/config"
	m "github.com/linkedin-inc/mane/model"
	t "github.com/linkedin-inc/mane/template"
	v "github.com/linkedin-inc/mane/vendor"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	UnsubscribeKeyword = "TD"
)

var (
	ErrInvalidStatus = errors.New("invalid status")
	ErrNothingPulled = errors.New("nothing pulled")
)

func Pull(name v.Name) error {
	vendors, err := v.GetByName(name)
	if err != nil {
		log.Error.Printf("occur error when find vendor %v : %v\n", name, err)
		return err
	}
	for _, vendor := range vendors {
		for {
			statuses, err := fetchStatus(vendor)
			if err != nil {
				log.Error.Printf("failed to pull status from %v : %v\n", vendor.Name(), err)
				break
			}
			if len(statuses) == 0 {
				break
			}
			processStatus(statuses)
		}
		for {
			replies, err := fetchReply(vendor)
			if err != nil {
				log.Error.Printf("failed to pull reply from %v : %v\n", vendor.Name(), err)
				break
			}
			if len(replies) == 0 {
				break
			}
			err = saveReply(replies)
			if err != nil {
				log.Error.Printf("failed to save reply from %v : %v\n", vendor.Name(), err)
				break
			}
		}
	}
	return nil
}

func fetchStatus(vendor v.Vendor) ([]m.DeliveryStatus, error) {
	statuses, err := vendor.Status()
	if err != nil {
		log.Error.Printf("failed to pull status from %v : %v\n", vendor.Name(), err)
		return []m.DeliveryStatus{}, err
	}
	err = saveStatus(statuses)
	if err != nil {
		log.Error.Printf("failed to save status from %v : %v\n", vendor.Name(), err)
	}
	return statuses, nil
}

func fetchReply(vendor v.Vendor) ([]m.Reply, error) {
	replies, err := vendor.Reply()
	if err != nil {
		log.Error.Printf("failed to pull reply from %v : %v\n", vendor.Name(), err)
		return []m.Reply{}, err
	}
	err = saveReply(replies)
	if err != nil {
		log.Error.Printf("failed to save reply from %v : %v\n", vendor.Name(), err)
		return []m.Reply{}, err
	}
	return replies, nil
}

func processStatus(statuses []m.DeliveryStatus) {
	if len(statuses) == 0 {
		return
	}
	checkedMsgIDs := []int64{}
	processedMsgIDs := []int64{}
	unprocessedMsgIDs := []int64{}
	//process status in loop! hmm, can process in batch?
	for _, status := range statuses {
		msgID, err := extractMsgID(status)
		if err != nil {
			continue
		}
		var history m.SMSHistory
		existed := mongodb.Exec(m.CollSMSHistory, func(c *mgo.Collection) error {
			return c.Find(bson.M{"msg_id": msgID}).One(&history)
		})
		if !existed {
			log.Error.Printf("missing original sms history, MsgID: %d\n", msgID)
			continue
		}
		var callback cb.Callback
		if t.Name(history.Template) == t.BlankName {
			category, err1 := c.WhichCategory(t.Category(history.Category))
			if err1 != nil {
				log.Error.Printf("failed to find category: %v\n", err1)
				continue
			}
			callback, err1 = cb.Lookup(category.Callback)
			if err1 != nil {
				log.Error.Printf("failed to lookup callback: %v\n", err1)
				unprocessedMsgIDs = append(unprocessedMsgIDs, msgID)
				continue
			}
			if callback == nil {
				checkedMsgIDs = append(checkedMsgIDs, msgID)
				continue
			}
		} else {
			template, err2 := c.WhichTemplate(t.Name(history.Template))
			if err2 != nil {
				log.Error.Printf("failed to find template: %v\n", err2)
				continue
			}
			callback, err2 = cb.Lookup(template.Callback)
			if err2 != nil {
				log.Error.Printf("failed to lookup callback: %v\n", err2)
				unprocessedMsgIDs = append(unprocessedMsgIDs, msgID)
				continue
			}
			if callback == nil {
				checkedMsgIDs = append(checkedMsgIDs, msgID)
				continue
			}
		}
		err = callback(&status, &history)
		if err != nil {
			log.Error.Printf("error when invoke callback: %v\n", err)
			//TODO retry or discard?
			unprocessedMsgIDs = append(unprocessedMsgIDs, msgID)
			continue
		}
		processedMsgIDs = append(processedMsgIDs, msgID)
	}
	//update state
	mongodb.ExecBulk(mongodb.GetMgoSession(), m.CollSMSHistory, func(b *mgo.Bulk) {
		params := []interface{}{
			bson.M{"msg_id": &bson.M{"$in": checkedMsgIDs}}, bson.M{"state": m.SMSStateChecked},
			bson.M{"msg_id": &bson.M{"$in": processedMsgIDs}}, bson.M{"state": m.SMSStateProcessed},
			bson.M{"msg_id": &bson.M{"$in": unprocessedMsgIDs}}, bson.M{"state": m.SMSStateUnprocessed},
		}
		b.Update(params...)
	})
}

func extractMsgID(status m.DeliveryStatus) (int64, error) {
	str := strconv.FormatInt(status.MsgID, 10)
	if len(str) != 15 && len(str) != 18 && len(str) != 19 {
		log.Error.Printf("status with invalid msg id: %d\n", status.MsgID)
		return int64(0), ErrInvalidStatus
	}
	return status.MsgID, nil
}

func saveStatus(statuses []m.DeliveryStatus) error {
	if len(statuses) == 0 {
		return nil
	}
	interfaces := make([]interface{}, len(statuses))
	for i, status := range statuses {
		interfaces[i] = status
	}
	var err error
	_ = mongodb.Exec(m.CollSMStatus, func(c *mgo.Collection) error {
		err = c.Insert(interfaces...)
		return err
	})
	return err
}

func saveReply(replies []m.Reply) error {
	if len(replies) == 0 {
		return nil
	}
	replyHistories := make([]interface{}, len(replies))
	var unsubscribers []interface{}
	for i, reply := range replies {
		replyHistories[i] = reply
		if strings.EqualFold(strings.TrimSpace(reply.Msg), UnsubscribeKeyword) {
			aUnsubscriber := m.Unsubscriber{
				Timestamp: reply.Timestamp,
				Phone:     reply.Phone,
			}
			unsubscribers = append(unsubscribers, aUnsubscriber)
		}
	}
	var err error
	mongodb.Cmd("chitu", func(db *mgo.Database) error {
		err = db.C(m.CollSMSReply).Insert(replyHistories...)
		if err != nil {
			return err
		}
		if len(unsubscribers) > 0 {
			err = db.C(m.CollUnsubscriber).Insert(unsubscribers...)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Error.Printf("occur error when save reply and unsubscriber: %v\n", err)
		return err
	}
	return nil
}
