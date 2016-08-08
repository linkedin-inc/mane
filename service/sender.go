package service

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	c "github.com/linkedin-inc/mane/config"
	"github.com/linkedin-inc/mane/logger"
	"github.com/linkedin-inc/mane/middleware"
	m "github.com/linkedin-inc/mane/model"
	t "github.com/linkedin-inc/mane/template"
	u "github.com/linkedin-inc/mane/util"
	v "github.com/linkedin-inc/mane/vendor"
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
	logger.I("executed to push sms, phones: %v, content: %v\n", phoneArray, content)
	vendor, err := v.GetByChannel(channel)
	if err != nil {
		logger.E("occur error when Push: %v\n", err)
		return "", err
	}
	seqID := generateSeqID()
	contentArray := []string{content}
	err = vendor.Send(strconv.FormatInt(seqID, 10), phoneArray, contentArray)
	if err != nil {
		if err == v.ErrNotInProduction {
			smsHistories := assembleHistory(phoneArray, content, seqID, channel, t.BlankName, category, vendor.Name(), m.SMSStateChecked)
			err := saveHistory(smsHistories)
			if err != nil {
				logger.E("failed to save Push: %v\n", err)
				return "", err
			}
			return strconv.FormatInt(seqID, 10), nil
		}
		logger.E("occur error when Push: %v\n", err)
		return "", err
	}
	smsHistories := assembleHistory(phoneArray, content, seqID, channel, t.BlankName, category, vendor.Name(), m.SMSStateUnchecked)
	err = saveHistory(smsHistories)
	if err != nil {
		logger.E("failed to save Push history: %v\n", err)
		return "", err
	}
	return strconv.FormatInt(seqID, 10), nil
}

//Batch sending group sms with different contents, will return the corresponding MsgID Array and the error
func MultiXPush(channel t.Channel, category t.Category, contentArray, phoneArray []string) ([]string, error) {
	if len(contentArray) != len(phoneArray) || len(contentArray) == 0 {
		return nil, ErrInvalidVariables
	}
	logger.I("executed to MultiXPush sms, phones: %v, content: %v\n", phoneArray, contentArray)
	vendor, err := v.GetByChannel(channel)
	if err != nil {
		logger.E("occur error when MultiXPush: %v\n", err)
		return []string{}, err
	}
	msgIDList := generateSeqIDList(len(contentArray))
	err = vendor.MultiXSend(msgIDList, phoneArray, contentArray)
	if err != nil {
		if err == v.ErrNotInProduction {
			smsHistories := assembleMultiHistory(phoneArray, contentArray, msgIDList, channel, t.BlankName, category, vendor.Name(), m.SMSStateChecked)
			err := saveHistory(smsHistories)
			if err != nil {
				logger.E("failed to save MultiXPush history: %v\n", err)
				return []string{}, err
			}
			return msgIDList, nil
		}
		logger.E("occur error when MultiXPush: %v\n", err)
		return []string{}, err
	}
	smsHistories := assembleMultiHistory(phoneArray, contentArray, msgIDList, channel, t.BlankName, category, vendor.Name(), m.SMSStateUnchecked)
	err = saveHistory(smsHistories)
	if err != nil {
		logger.E("failed to save MultiXPush history: %v\n", err)
		return []string{}, err
	}
	return msgIDList, nil
}

func send(name t.Name, variables map[string]string, allowed []string) (string, string, error) {
	template, err := c.WhichTemplate(name)
	if err != nil {
		logger.E("occur error when send sms: %v\n", err)
		return "", "", err
	}
	channel, err := c.WhichChannel(template.Category)
	if err != nil {
		logger.E("occur error when send sms: %v\n", err)
		return "", "", err
	}
	vendor, err := v.GetByChannel(channel)
	if err != nil {
		logger.E("occur error when send sms: %v\n", err)
		return "", "", err
	}
	logger.I("template: %v\n", template.Content)
	content, err := assembleTemplate(template.Content, variables)
	if err != nil {
		logger.E("occur error when send sms: %v\n", err)
		return "", "", err
	}
	if content == "" {
		return "", "", ErrInvalidContent
	}
	logger.I("content: %v\n", content)
	seqID := generateSeqID()
	contentArray := []string{content}
	err = vendor.Send(strconv.FormatInt(seqID, 10), allowed, contentArray)
	if err != nil {
		if err == v.ErrNotInProduction {
			smsHistories := assembleHistory(allowed, content, seqID, channel, name, template.Category, vendor.Name(), m.SMSStateChecked)
			err := saveHistory(smsHistories)
			if err != nil {
				logger.E("failed to save sms history: %v\n", err)
				return "", "", err
			}
			return strconv.FormatInt(seqID, 10), content, nil
		}
		logger.E("occur error when send sms: %v\n", err)
		return "", "", err
	}
	smsHistories := assembleHistory(allowed, content, seqID, channel, name, template.Category, vendor.Name(), m.SMSStateUnchecked)
	err = saveHistory(smsHistories)
	if err != nil {
		logger.E("failed to save sms history: %v\n", err)
		return "", "", err
	}
	return strconv.FormatInt(seqID, 10), content, nil
}

//Send normal sms to phones with given template and variables, will return MsgID, content and optional error
func Send(name t.Name, variables map[string]string, phoneArray []string, actions ...middleware.Action) (string, string, error) {
	logger.I("executed to send sms, phones: %v, template: %v\n", phoneArray, name)
	if len(phoneArray) == 0 {
		return "", "", ErrInvalidPhoneArray
	}
	if len(variables) == 0 {
		return "", "", ErrInvalidVariables
	}
	var contexts []m.SMSContext
	for i := 0; i < len(phoneArray); i++ {
		contexts = append(contexts, *m.NewSMSContext(phoneArray[i], string(name), variables))
	}
	allowedContexts := middleware.NewMiddleware(actions...).Call(contexts)
	if len(allowedContexts) == 0 {
		return "", "", ErrNotAllowed
	}
	allowed := make([]string, len(allowedContexts))
	for i := 0; i < len(allowedContexts); i++ {
		allowed[i] = allowedContexts[i].Phone
	}
	return send(name, variables, allowed)
}

func multiXSend(name t.Name, allowedVariableArray []map[string]string, allowed []string) ([]string, []string, []string, error) {
	template, err := c.WhichTemplate(name)
	if err != nil {
		logger.E("occur error when MultiXSend sms: %v\n", err)
		return nil, nil, nil, err
	}
	channel, err := c.WhichChannel(template.Category)
	if err != nil {
		logger.E("occur error when MultiXSend sms: %v\n", err)
		return nil, nil, nil, err
	}
	vendor, err := v.GetByChannel(channel)
	if err != nil {
		logger.E("occur error when MultiXSend sms: %v\n", err)
		return nil, nil, nil, err
	}
	logger.I("template: %v\n", template.Content)
	contentArray, err := assembleTemplateArray(template.Content, allowedVariableArray)
	if err != nil {
		logger.E("occur error when MultiXSend sms: %v\n", err)
		return nil, nil, nil, err
	}
	msgIDList := generateSeqIDList(len(allowed))
	err = vendor.MultiXSend(msgIDList, allowed, contentArray)
	if err != nil {
		if err == v.ErrNotInProduction {
			smsHistories := assembleMultiHistory(allowed, contentArray, msgIDList, channel, name, template.Category, vendor.Name(), m.SMSStateChecked)
			err := saveHistory(smsHistories)
			if err != nil {
				logger.E("failed to save MultiXSend history: %v\n", err)
				return nil, nil, nil, err
			}
			return msgIDList, allowed, contentArray, nil
		}
		logger.E("occur error when MultiXSend sms: %v\n", err)
		return nil, nil, nil, err
	}
	smsHistories := assembleMultiHistory(allowed, contentArray, msgIDList, channel, name, template.Category, vendor.Name(), m.SMSStateUnchecked)
	err = saveHistory(smsHistories)
	if err != nil {
		logger.E("failed to save MultiXSend history: %v\n", err)
		return nil, nil, nil, err
	}
	return msgIDList, allowed, contentArray, nil
}

// batch send sms with different values map for one tpl, return  msgid array, allowed phone array, content array and the error
func MultiXSend(name t.Name, variableArray []map[string]string, phoneArray []string, actions ...middleware.Action) ([]string, []string, []string, error) {
	logger.I("executed to MultiXSend sms, phones: %v, template: %v\n", phoneArray, name)
	if len(phoneArray) == 0 {
		return nil, nil, nil, ErrInvalidPhoneArray
	}
	if len(variableArray) == 0 || len(phoneArray) != len(variableArray) {
		return nil, nil, nil, ErrInvalidVariables
	}

	phone2Var := make(map[string]map[string]string)
	var contexts []m.SMSContext
	for i := 0; i < len(phoneArray); i++ {
		contexts = append(contexts, *m.NewSMSContext(phoneArray[i], string(name), variableArray[i]))
		phone2Var[phoneArray[i]] = variableArray[i]
	}
	allowedContexts := middleware.NewMiddleware(actions...).Call(contexts)
	if len(allowedContexts) == 0 {
		return nil, nil, nil, ErrNotAllowed
	}
	allowedVariableArray := make([]map[string]string, len(allowedContexts))
	allowed := make([]string, len(allowedContexts))
	for i := 0; i < len(allowedContexts); i++ {
		allowedVariableArray[i] = phone2Var[allowedContexts[i].Phone]
		allowed[i] = allowedContexts[i].Phone
	}
	return multiXSend(name, allowedVariableArray, allowed)
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

func assembleMultiHistory(phoneArray []string, contentArray []string, seqIDStrArray []string, channel t.Channel, template t.Name, category t.Category, vendor v.Name, state m.SMSState) []interface{} {
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
	return saver.Save(m.CollSMSHistory, histories)
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

func assembleTemplateArray(content string, variableArray []map[string]string) ([]string, error) {
	var assembledArray []string
	for _, v := range variableArray {
		var variablesArray []string
		for key, value := range v {
			variablesArray = append(variablesArray, fmt.Sprintf(variableWrapper, key), value)
		}
		if len(variablesArray)%2 == 1 {
			return []string{}, ErrInvalidVariables
		}
		replacer := strings.NewReplacer(variablesArray...)
		assembled := replacer.Replace(content)
		assembledArray = append(assembledArray, assembled)
	}
	return assembledArray, nil
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
