package service

import (
	"encoding/json"
	"fmt"
	"linkedin/log"
	"linkedin/service/mongodb"
	"linkedin/service/neo4j"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/linkedin-inc/go-workers"
	c "github.com/linkedin-inc/mane/config"
	f "github.com/linkedin-inc/mane/filter"
	m "github.com/linkedin-inc/mane/model"
	t "github.com/linkedin-inc/mane/template"
	u "github.com/linkedin-inc/mane/util"
	v "github.com/linkedin-inc/mane/vendor"
	"github.com/linkedin-inc/neoism"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	variableWrapper      = "{%s}"
	ErrInvalidVariables  = errors.New("invalid variables")
	ErrCannotTrack       = errors.New("cannot track")
	ErrInvalidContent    = errors.New("invalid content")
	ErrInvalidPhoneArray = errors.New("invalid phone array")
	ErrNotAllowed        = errors.New("not allowed")
)

//Push sms to phones directly with given content, will return MsgID and optional error
func Push(channel t.Channel, category t.Category, content string, phoneArray []string) (string, error) {
	log.Info.Printf("executed to push sms, phones: %v, content: %v\n", phoneArray, content)
	vendor, err := v.GetByChannel(channel)
	if err != nil {
		log.Error.Printf("occur error when send sms: %v\n", err)
		return "", err
	}
	seqID := generateSeqID()
	contentArray := []string{content}
	err = vendor.Send(strconv.FormatInt(seqID, 10), phoneArray, contentArray)
	if err != nil {
		if err == v.ErrNotInProduction {
			go func() {
				smsHistories := assembleHistory(phoneArray, content, seqID, channel, t.BlankName, category, vendor.Name(), m.SMSStateChecked)
				err := saveHistory(smsHistories)
				if err != nil {
					log.Error.Printf("failed to save sms history: %v\n", err)
				}
			}()
			return strconv.FormatInt(seqID, 10), nil
		}
		log.Error.Printf("occur error when send sms: %v\n", err)
		return "", err
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "[mane] err after push %v", r)
			}
		}()
		updateLastEngagement(phoneArray)
		smsHistories := assembleHistory(phoneArray, content, seqID, channel, t.BlankName, category, vendor.Name(), m.SMSStateUnchecked)
		err := saveHistory(smsHistories)
		if err != nil {
			log.Error.Printf("failed to save sms history: %v\n", err)
		}
	}()
	return strconv.FormatInt(seqID, 10), nil
}

//Batch sending group sms with different contents, will return the corresponding MsgID Array and the error
func MultiXPush(channel t.Channel, category t.Category, contentArray, phoneArray []string) ([]string, error) {
	if len(contentArray) != len(phoneArray) || len(contentArray) == 0 {
		return []string{}, ErrInvalidVariables
	}
	fmt.Printf("executed to MultiPush sms, phones: %v, content: %v\n", phoneArray[0], contentArray[0])
	vendor, err := v.GetByChannel(channel)
	if err != nil {
		fmt.Fprintln(os.Stderr, "occur error when send sms: %v\n", err)
		return []string{}, err
	}
	msgIDList := generateSeqIDList(len(contentArray))
	err = vendor.MultiXSend(msgIDList, phoneArray, contentArray)
	if err != nil {
		if err == v.ErrNotInProduction {
			go func() {
				smsHistories := assembleMultiXHistory(phoneArray, contentArray, msgIDList, channel, t.BlankName, category, vendor.Name(), m.SMSStateChecked)
				err := saveHistory(smsHistories)
				if err != nil {
					fmt.Fprintln(os.Stderr, "failed to save multix sms history: %v\n", err)
				}
			}()
			return msgIDList, nil
		}
		log.Error.Printf("occur error when send multix sms: %v\n", err)
		return []string{}, err
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "[mane] err after multix push %v", r)
			}
		}()
		updateLastEngagement(phoneArray)
		smsHistories := assembleMultiXHistory(phoneArray, contentArray, msgIDList, channel, t.BlankName, category, vendor.Name(), m.SMSStateChecked)
		err := saveHistory(smsHistories)
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to save multix sms history: %v\n", err)
		}
	}()
	return msgIDList, nil
}

//Trigger by SMS job, such as postpone worker
func Trigger(msg *workers.Msg) {
	jsonStr := msg.Args().ToJson()
	log.Info.Printf("triggered to send sms, msg: %v\n", jsonStr)
	var job m.SMSJob
	if err := json.Unmarshal([]byte(jsonStr), &job); err != nil {
		log.Error.Printf("discard due to parse sms job failed: %v\n", err)
		return
	}
	_, _, err := Send(t.Name(job.Template), job.Variables, []string{job.Phone})
	if err != nil && err != ErrNotAllowed {
		log.Error.Printf("occur error when trigger to send sms: %v\n", err)
	}
}

//Send normal sms to phones with given template and variables, will return MsgID, content and optional error
func Send(name t.Name, variables map[string]string, phoneArray []string) (string, string, error) {
	log.Info.Printf("executed to send sms, phones: %v, template: %v\n", phoneArray, name)
	if len(phoneArray) == 0 {
		return "", "", ErrInvalidPhoneArray
	}
	if len(variables) == 0 {
		return "", "", ErrInvalidVariables
	}
	f.StoreVariables(phoneArray, name, variables)
	allowed := f.ProcessChain(phoneArray, name)
	f.ClearVariables(phoneArray, name)
	if len(allowed) == 0 {
		return "", "", ErrNotAllowed
	}
	template, err := c.WhichTemplate(name)
	if err != nil {
		log.Error.Printf("occur error when send sms: %v\n", err)
		return "", "", err
	}
	channel, err := c.WhichChannel(template.Category)
	if err != nil {
		log.Error.Printf("occur error when send sms: %v\n", err)
		return "", "", err
	}
	vendor, err := v.GetByChannel(channel)
	if err != nil {
		log.Error.Printf("occur error when send sms: %v\n", err)
		return "", "", err
	}
	log.Info.Printf("template: %v\n", template.Content)
	content, err := assembleTemplate(template.Content, variables)
	if err != nil {
		log.Error.Printf("occur error when send sms: %v\n", err)
		return "", "", err
	}
	if content == "" {
		return "", "", ErrInvalidContent
	}
	log.Info.Printf("content: %v\n", content)
	seqID := generateSeqID()
	contentArray := []string{content}
	err = vendor.Send(strconv.FormatInt(seqID, 10), allowed, contentArray)
	if err != nil {
		if err == v.ErrNotInProduction {
			go func() {
				smsHistories := assembleHistory(allowed, content, seqID, channel, name, template.Category, vendor.Name(), m.SMSStateChecked)
				err := saveHistory(smsHistories)
				if err != nil {
					log.Error.Printf("failed to save sms history: %v\n", err)
				}
			}()
			return strconv.FormatInt(seqID, 10), content, nil
		}
		log.Error.Printf("occur error when send sms: %v\n", err)
		return "", "", err
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "[mane] err after push %v", err)
			}
		}()
		//FIXME update timestamp of last engagament to user?
		updateLastEngagement(phoneArray)
		smsHistories := assembleHistory(allowed, content, seqID, channel, name, template.Category, vendor.Name(), m.SMSStateUnchecked)
		err := saveHistory(smsHistories)
		if err != nil {
			log.Error.Printf("failed to save sms history: %v\n", err)
		}
	}()
	return strconv.FormatInt(seqID, 10), content, nil
}

func generateSeqID() int64 {
	timestamp := time.Now().UnixNano()
	r := rand.New(rand.NewSource(timestamp))
	seqID := timestamp/1e6*100 + r.Int63n(99)
	return seqID
}

func generateSeqIDList(length int) []string {
	seqIDList := make([]string, length)
	for i := 0; i < length; i++ {
		seqIDList[i] = strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return seqIDList
}

func assembleHistory(phoneArray []string, content string, seqID int64, channel t.Channel, template t.Name, category t.Category, vendor v.Name, state m.SMSState) []interface{} {
	timestamp := time.Now()
	docs := make([]interface{}, len(phoneArray))
	for i, phone := range phoneArray {
		sms := m.SMSHistory{
			MsgID:     seqID,
			Timestamp: timestamp,
			Phone:     phone,
			Content:   content,
			Template:  string(template),
			Category:  string(category),
			Channel:   int(channel),
			Vendor:    string(vendor),
			State:     state,
		}
		docs[i] = sms
	}
	return docs
}

func assembleMultiXHistory(phoneArray []string, contentArray []string, seqIDStrArray []string, channel t.Channel, template t.Name, category t.Category, vendor v.Name, state m.SMSState) []interface{} {
	timestamp := time.Now()
	docs := make([]interface{}, len(phoneArray))
	for i := range phoneArray {
		sms := m.SMSHistory{
			MsgID:     u.Atoi64Safe(seqIDStrArray[i], 0),
			Timestamp: timestamp,
			Phone:     phoneArray[i],
			Content:   contentArray[i],
			Template:  string(template),
			Category:  string(category),
			Channel:   int(channel),
			Vendor:    string(vendor),
			State:     state,
		}
		docs[i] = sms
	}
	return docs
}

func saveHistory(histories []interface{}) error {
	if len(histories) == 0 {
		return nil
	}
	var err error
	_ = mongodb.Exec(m.CollSMSHistory, func(c *mgo.Collection) error {
		err = c.Insert(histories...)
		return err
	})
	return err
}

func updateLastEngagement(phoneArray []string) {
	for _, phone := range phoneArray {
		ret := []m.TinyUser{}
		neo4j.READONLY.C(`MATCH (n:User) WHERE n.phone={phone} RETURN id(n) as _id, n.phone as phone`,
			neoism.Props{"phone": phone},
			&ret,
		)
		if len(ret) > 0 {
			if !mongodb.Exec("user", func(c *mgo.Collection) error {
				return c.Update(bson.M{"_id": ret[0].UserID}, bson.M{"$set": bson.M{"last_receive_sms": u.CurrentTimeMillis()}})
			}) {
				log.Info.Println("user not exist")
			}
		}
	}
}

func assembleTemplate(content string, variables map[string]string) (string, error) {
	//TODO how to deal with trackable sms
	//trackable, err := isTrackable(content, variables)
	//if err != nil {
	//	return "", err
	//}
	var variablesArray []string
	for key, value := range variables {
		//wrap key with curly braces. for example, key is 'name' and wrapped as '{name}'
		variablesArray = append(variablesArray, fmt.Sprintf(variableWrapper, key), value)
	}
	if len(variablesArray)%2 == 1 {
		return "", ErrInvalidVariables
	}
	replacer := strings.NewReplacer(variablesArray...)
	assembled := replacer.Replace(content)
	return assembled, nil
}

func isTrackable(content string, variables map[string]string) (bool, error) {
	if strings.Contains(content, "link") || strings.Contains(content, "url") {
		_, containsUserID := variables["userid"]
		if containsUserID {
			return true, nil
		}
		return false, ErrCannotTrack
	}
	return false, nil
}
