package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	c "github.com/linkedin-inc/mane/config"
	"github.com/linkedin-inc/mane/logger"
	"github.com/linkedin-inc/mane/middleware"
	m "github.com/linkedin-inc/mane/model"
	t "github.com/linkedin-inc/mane/template"
	v "github.com/linkedin-inc/mane/vendor"
)

var (
	variableWrapper      = "{%s}"
	ErrInvalidVariables  = errors.New("invalid variables")
	ErrInvalidPhoneArray = errors.New("invalid phone array")
	ErrNotAllowed        = errors.New("not allowed")
)

// NOTE: each template and variables in context must be the same, and the id field must be unique and not empty
func Send(contexts []*m.SMSContext, actions ...middleware.Action) error {
	if len(contexts) == 0 {
		return ErrInvalidPhoneArray
	}
	allowedContexts := middleware.NewMiddleware(actions...).Call(contexts)
	if len(allowedContexts) == 0 {
		return ErrNotAllowed
	}
	vendor, err := assembleMetaData(contexts)
	if err != nil {
		logger.E("occur error when MultiXSend sms: %v\n", err)
		return err
	}
	err = vendor.Send(contexts)
	if err != nil && err != v.ErrNotInProduction {
		return err
	}
	return nil
}

// NOTE: each template in context must be the same, and the id field must be unique and not empty
func MultiXSend(contexts []*m.SMSContext, actions ...middleware.Action) error {
	if len(contexts) == 0 {
		return ErrInvalidPhoneArray
	}
	allowedContexts := middleware.NewMiddleware(actions...).Call(contexts)
	if len(allowedContexts) == 0 {
		return ErrNotAllowed
	}
	vendor, err := assembleMultiMetaData(contexts)
	if err != nil {
		logger.E("occur error when MultiXSend sms: %v\n", err)
		return err
	}
	err = vendor.MultiXSend(contexts)
	if err != nil && err != v.ErrNotInProduction {
		return err
	}
	return nil
}

func assembleMetaData(contexts []*m.SMSContext) (v.Vendor, error) {
	template, err := c.WhichTemplate(t.Name(contexts[0].Template))
	if err != nil {
		logger.E("occur error when assembleMetaData: %v\n", err)
		return nil, err
	}
	channel, err := c.WhichChannel(template.Category)
	if err != nil {
		logger.E("occur error when assembleMetaData: %v\n", err)
		return nil, err
	}
	vendor, err := v.GetByChannel(channel)
	if err != nil {
		logger.E("occur error when assembleMetaData: %v\n", err)
		return nil, err
	}

	// generate msgid list and contents
	msgID := m.NewSmsContextID()

	var variablesArray []string
	for key, value := range contexts[0].Variables {
		variablesArray = append(variablesArray, fmt.Sprintf(variableWrapper, key), value)
	}
	if len(variablesArray)%2 == 1 {
		return nil, ErrInvalidVariables
	}
	replacer := strings.NewReplacer(variablesArray...)
	content := replacer.Replace(template.Content)

	for i := range contexts {
		contexts[i].History = &m.SMSHistory{
			ID:        contexts[i].ID,
			MsgID:     msgID,
			Timestamp: time.Now(),
			Phone:     contexts[i].Phone,
			Content:   content,
			Template:  contexts[i].Template,
			Category:  string(template.Category),
			Channel:   int(channel),
			Vendor:    string(vendor.Name()),
			State:     m.SMSStateUnchecked,
		}
	}
	return vendor, nil
}

func assembleMultiMetaData(contexts []*m.SMSContext) (v.Vendor, error) {
	template, err := c.WhichTemplate(t.Name(contexts[0].Template))
	if err != nil {
		logger.E("occur error when assembleMultiMetaData: %v\n", err)
		return nil, err
	}
	channel, err := c.WhichChannel(template.Category)
	if err != nil {
		logger.E("occur error when assembleMultiMetaData: %v\n", err)
		return nil, err
	}
	vendor, err := v.GetByChannel(channel)
	if err != nil {
		logger.E("occur error when assembleMultiMetaData: %v\n", err)
		return nil, err
	}

	// generate msgid list and contents
	msgIDList := make([]int64, len(contexts))
	contentList := make([]string, len(contexts))
	for i := range contexts {
		msgIDList[i] = m.NewSmsContextID()

		var variablesArray []string
		for key, value := range contexts[i].Variables {
			variablesArray = append(variablesArray, fmt.Sprintf(variableWrapper, key), value)
		}
		if len(variablesArray)%2 == 1 {
			return nil, ErrInvalidVariables
		}
		replacer := strings.NewReplacer(variablesArray...)
		assembled := replacer.Replace(template.Content)
		contentList[i] = assembled
	}

	for i := range contexts {
		contexts[i].History = &m.SMSHistory{
			ID:        contexts[i].ID,
			MsgID:     msgIDList[i],
			Timestamp: time.Now(),
			Phone:     contexts[i].Phone,
			Content:   contentList[i],
			Template:  contexts[i].Template,
			Category:  string(template.Category),
			Channel:   int(channel),
			Vendor:    string(vendor.Name()),
			State:     m.SMSStateUnchecked,
		}
	}
	return vendor, nil
}
