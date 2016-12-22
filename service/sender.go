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
	v "github.com/linkedin-inc/mane/vendors"
)

var (
	variableWrapper      = "{%s}"
	ErrInvalidVariables  = errors.New("invalid variables")
	ErrInvalidPhoneArray = errors.New("invalid phone array")
	ErrNotAllowed        = errors.New("not allowed")
)

// single sms send interface
func Send(context *m.SMSContext) (*m.SMSContext, error) {
	if context == nil {
		return nil, ErrInvalidPhoneArray
	}
	allowedContext, vendor, err := assembleMetaData(context)
	if err != nil {
		logger.E("occur error when Send sms: %v\n", err)
		return nil, err
	}
	err = vendor.Send(allowedContext)
	if err != nil && err != v.ErrNotInProduction {
		return nil, err
	}
	return allowedContext, nil
}

// NOTE: each template in context must be the same, and the id field must be unique and not empty
func MultiXSend(contexts []*m.SMSContext) ([]*m.SMSContext, error) {
	if len(contexts) == 0 {
		return nil, ErrInvalidPhoneArray
	}

	allowedContexts, vendor, err := assembleMultiMetaData(contexts)
	if err != nil {
		logger.E("occur error when MultiXSend sms: %v\n", err)
		return nil, err
	}
	succeedContexts, err := vendor.MultiXSend(allowedContexts)
	if err == nil || err == v.ErrNotInProduction {
		return succeedContexts, nil
	}
	return succeedContexts, err
}

func assembleMetaData(context *m.SMSContext) (*m.SMSContext, v.Vendor, error) {
	template, err := c.WhichTemplate(t.Name(context.Template))
	if err != nil {
		logger.E("occur error when assembleMetaData: %v\n", err)
		return nil, nil, err
	}
	allowedContexts := middleware.NewMiddleware(template.ActionList...).Call([]*m.SMSContext{context})
	if len(allowedContexts) == 0 {
		return nil, nil, ErrNotAllowed
	}
	channel, err := c.WhichChannel(template.Category)
	if err != nil {
		logger.E("occur error when assembleMetaData: %v\n", err)
		return nil, nil, err
	}
	vendor, err := v.GetByChannel(channel)
	if err != nil {
		logger.E("occur error when assembleMetaData: %v\n", err)
		return nil, nil, err
	}

	// generate msgid list and contents
	msgID := m.NewSmsContextID()
	var variablesArray []string
	for key, value := range context.Variables {
		variablesArray = append(variablesArray, fmt.Sprintf(variableWrapper, key), value)
	}
	if len(variablesArray)%2 == 1 {
		return nil, nil, ErrInvalidVariables
	}
	replacer := strings.NewReplacer(variablesArray...)
	content := replacer.Replace(template.Content)
	context.History = &m.SMSHistory{
		MID:       context.ID,
		MsgID:     msgID,
		Timestamp: time.Now(),
		Phone:     context.Phone,
		Content:   content,
		Template:  context.Template,
		Category:  string(template.Category),
		Channel:   int(channel),
		Vendor:    string(vendor.Name()),
		State:     m.SMSStateUnchecked,
	}
	return context, vendor, nil
}

func assembleMultiMetaData(contexts []*m.SMSContext) ([]*m.SMSContext, v.Vendor, error) {
	template, err := c.WhichTemplate(t.Name(contexts[0].Template))
	if err != nil {
		logger.E("occur error when assembleMultiMetaData: %v\n", err)
		return nil, nil, err
	}
	allowedContexts := middleware.NewMiddleware(template.ActionList...).Call(contexts)
	if len(allowedContexts) == 0 {
		return nil, nil, ErrNotAllowed
	}
	channel, err := c.WhichChannel(template.Category)
	if err != nil {
		logger.E("occur error when assembleMultiMetaData: %v\n", err)
		return nil, nil, err
	}
	vendor, err := v.GetByChannel(channel)
	if err != nil {
		logger.E("occur error when assembleMultiMetaData: %v\n", err)
		return nil, nil, err
	}

	// generate msgid list and contents
	msgIDList := make([]int64, len(allowedContexts))
	contentList := make([]string, len(allowedContexts))
	for i := range allowedContexts {
		msgIDList[i] = m.NewSmsContextID()

		var variablesArray []string
		for key, value := range allowedContexts[i].Variables {
			variablesArray = append(variablesArray, fmt.Sprintf(variableWrapper, key), value)
		}
		if len(variablesArray)%2 == 1 {
			return nil, nil, ErrInvalidVariables
		}
		replacer := strings.NewReplacer(variablesArray...)
		assembled := replacer.Replace(template.Content)
		contentList[i] = assembled
	}

	for i := range allowedContexts {
		allowedContexts[i].History = &m.SMSHistory{
			MID:       allowedContexts[i].ID,
			MsgID:     msgIDList[i],
			Timestamp: time.Now(),
			Phone:     allowedContexts[i].Phone,
			Content:   contentList[i],
			Template:  allowedContexts[i].Template,
			Category:  string(template.Category),
			Channel:   int(channel),
			Vendor:    string(vendor.Name()),
			State:     m.SMSStateUnchecked,
		}
	}
	return allowedContexts, vendor, nil
}
