package vendor

import (
	"encoding/base64"
	"encoding/xml"
	"errors"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/axgle/mahonia"
	"github.com/linkedin-inc/mane/logger"
	mo "github.com/linkedin-inc/mane/model"
	u "github.com/linkedin-inc/mane/util"
)

const (
	formKeyMsgID       = "MsgId"
	formKeyUserName    = "userId"
	formKeyPassword    = "password"
	formKeyPhoneArray  = "pszMobis"
	formKeyMessage     = "pszMsg"
	formKeyPhoneCount  = "iMobiCount"
	formKeySubPort     = "pszSubPort"
	formKeyRequestType = "iReqType"
	formMultixmt       = "multixmt"
	requestTypeReply   = "1"
	requestTypeStatus  = "2"
	maxSendNumEachTime = 100 // limited by the vendor
	poolSize           = 10
	retryTimes         = 4
)

var (
	NameMontnets  = Name("montnets")
	errorCode2Msg = map[string]string{
		"-1":     "参数为空",
		"-12":    "有异常电话号码",
		"-14":    "实际号码个数超过100",
		"-999":   "服务器内部错误",
		"-10001": "用户登录不成功",
		"-10003": "用户余额不足",
		"-10011": "信息内容超长",
		"-10029": "此用户没有权限从此通道发送消息",
		"-10030": "不能发送移动号码",
		"-10031": "手机号码(段)非法",
		"-10057": "IP受限",
		"-10056": "连接数超限",
	}
)

type montnetsSendResponse struct {
	Result string `xml:"string"`
}

type montnetsUpstreamResponse struct {
	Result []string `xml:"string"`
}

type Montnets struct {
	Username        string
	Password        string
	SendEndpoint    string
	MultiXSendPoint string
	StatusEndpoint  string
	BalanceEndpoint string
}

func NewMontnets(username, password, sendEndpoint, statusEndpoint, balanceEndpoint, multiXSendPoint string) Montnets {
	return Montnets{
		Username:        username,
		Password:        password,
		SendEndpoint:    sendEndpoint,
		StatusEndpoint:  statusEndpoint,
		BalanceEndpoint: balanceEndpoint,
		MultiXSendPoint: multiXSendPoint,
	}
}

func (m Montnets) Name() Name {
	return NameMontnets
}

//Send sms to given phone number with content
func (m Montnets) Send(contexts []*mo.SMSContext) ([]*mo.SMSContext, error) {
	//TODO we should ensure all content must be the same
	//only send in production environment
	if !u.IsProduction() {
		logger.E("discard due to not in production environment!")
		return contexts, ErrNotInProduction
	}
	var succeedContexts []*mo.SMSContext
	phoneArray := m.extractPhoneArray(contexts)
	msgID := strconv.FormatInt(contexts[0].History.MsgID, 10)
	content := contexts[0].History.Content

	pool := u.NewPool(poolSize, poolSize)
	defer pool.Release()
	jobCount := int(math.Ceil(float64(len(phoneArray)) / float64(maxSendNumEachTime))) // total job count
	pool.WaitCount(jobCount)
	logger.I("start sending sms, total length: %d, total job count: %d", len(phoneArray), jobCount)
	for i := 0; i < jobCount; i++ {
		start := i * maxSendNumEachTime
		end := start + maxSendNumEachTime
		currentStep := i
		pool.JobQueue <- func() {
			defer func() {
				if r := recover(); r != nil {
					logger.E("err:%v\n", r)
				}
				pool.JobDone()
			}()
			if end > len(phoneArray) {
				end = len(phoneArray)
			}
			if start >= end {
				return
			}
			var response *http.Response
			var err error
			for i := 0; i < retryTimes; i++ {
				logger.D("start sending sms, current step:%d, start:%d, end:%d, retryTimes:%d", currentStep, start, end, i)
				request := m.assembleSendRequest(msgID, phoneArray[start:end], content)
				response, err = http.PostForm(m.SendEndpoint, *request)
				if err != nil {
					logger.E("retryTimes:%d, failed to send sms[%d:%d]: %v\n", i, start, end, err)
					if i == retryTimes-1 {
						return
					}
					time.Sleep(time.Second)
				} else {
					break
				}
			}
			if s := response.StatusCode; s != http.StatusOK {
				return
			}
			err = m.handleSendResponse(response)
			if err != nil {
				logger.E("failed to handle send response[%d:%d]: %v\n", start, end, err)
				return
			}
			succeedContexts = append(succeedContexts, contexts[start:end]...)
		}
	}
	pool.WaitAll()
	logger.I("finish sending sms, total count: %d, succeed count: %d\n", len(contexts), len(succeedContexts))
	return succeedContexts, nil
}

func (m Montnets) assembleSendRequest(seqID string, phoneArray []string, content string) *url.Values {
	form := url.Values{}
	form.Add(formKeyUserName, m.Username)
	form.Add(formKeyPassword, m.Password)
	form.Add(formKeyPhoneArray, strings.Join(phoneArray, ","))
	form.Add(formKeyMessage, content)
	form.Add(formKeyPhoneCount, strconv.Itoa(len(phoneArray)))
	form.Add(formKeySubPort, "*")
	form.Add(formKeyMsgID, seqID)
	return &form
}

func (m Montnets) handleSendResponse(response *http.Response) error {
	defer func() {
		_ = response.Body.Close()
	}()
	data, _ := ioutil.ReadAll(response.Body)
	//actually we just ignore what the response is exactly, later we will check delivery status of messages.
	//matched := responseMatcher.FindSubmatch(data)
	//if matched != nil && len(matched) == 2 {
	//	return nil
	//}
	//return errSendFailed
	var body montnetsSendResponse
	err := xml.Unmarshal(data, &body)
	if err != nil {
		//omit error
		return nil
	}
	errMsg, existed := errorCode2Msg[body.Result]
	if existed {
		return errors.New(errMsg)
	}
	return nil
}

func (m Montnets) Status() ([]*mo.DeliveryStatus, error) {
	request := m.assembleUpstreamRequest(requestTypeStatus)
	response, err := http.PostForm(m.StatusEndpoint, *request)
	if err != nil {
		logger.E("failed to check status: %v\n", err)
		return nil, ErrGetStatusFailed
	}
	if s := response.StatusCode; s != http.StatusOK {
		return nil, ErrGetStatusFailed
	}
	status, err := m.handleUpstreamResponse(response)
	if err != nil {
		logger.E("failed to handle status response: %v\n", err)
		return nil, ErrGetStatusFailed
	}
	var parsedStatus []*mo.DeliveryStatus
	if len(status) == 0 {
		return parsedStatus, nil
	}
	parsedStatus = m.parseStatus(status)
	return parsedStatus, nil
}

func (m Montnets) assembleUpstreamRequest(requestType string) *url.Values {
	form := url.Values{}
	form.Add(formKeyUserName, m.Username)
	form.Add(formKeyPassword, m.Password)
	form.Add(formKeyRequestType, requestType)
	return &form
}

func (m Montnets) handleUpstreamResponse(response *http.Response) ([]string, error) {
	defer func() {
		_ = response.Body.Close()
	}()
	data, _ := ioutil.ReadAll(response.Body)
	var body montnetsUpstreamResponse
	err := xml.Unmarshal(data, &body)
	if err != nil {
		return nil, err
	}
	return body.Result, nil
}

func (m Montnets) parseStatus(raw []string) []*mo.DeliveryStatus {
	var statuses []*mo.DeliveryStatus
	for _, rawRecord := range raw {
		splited := strings.Split(rawRecord, ",")
		// avoid out of range panic
		if len(splited) != 9 {
			logger.E("err response:%s\n", rawRecord)
			continue
		}
		timestamp, err := time.ParseInLocation("2006-01-02 15:04:05", splited[1], time.Local)
		if err != nil {
			logger.E("failed to parse time: %v\n", err)
			//discard and go ahead
			continue
		}
		status := &mo.DeliveryStatus{
			MsgID:      u.Atoi64Safe(splited[5], -1),
			Timestamp:  timestamp,
			Phone:      splited[4],
			StatusCode: u.Atoi32Safe(splited[7], -1),
		}
		//omit fixed detail msg like 'DELIVERED'
		if splited[7] != "0" {
			status.ErrorMsg = splited[8]
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func (m Montnets) Reply() ([]*mo.Reply, error) {
	request := m.assembleUpstreamRequest(requestTypeReply)
	response, err := http.PostForm(m.StatusEndpoint, *request)
	if err != nil {
		logger.E("failed to get reply: %v\n", err)
		return nil, ErrGetReplyFailed
	}
	if s := response.StatusCode; s != http.StatusOK {
		return nil, ErrGetReplyFailed
	}
	replies, err := m.handleUpstreamResponse(response)
	if err != nil {
		logger.E("failed to handle reply response: %v\n", err)
		return nil, ErrGetReplyFailed
	}
	var parsedReplies []*mo.Reply
	if len(replies) == 0 {
		return parsedReplies, nil
	}
	parsedReplies = m.parseReply(replies)
	return parsedReplies, nil
}

func (m Montnets) parseReply(raw []string) []*mo.Reply {
	var replies []*mo.Reply
	for _, rawRecord := range raw {
		splited := strings.Split(rawRecord, ",")
		timestamp, err := time.ParseInLocation("2006-01-02 15:04:05", splited[1], time.Local)
		if err != nil {
			logger.E("failed to parse time: %v\n", err)
			//discard and go ahead
			continue
		}
		reply := &mo.Reply{
			Timestamp: timestamp,
			Phone:     splited[2],
			Msg:       strings.TrimSpace(splited[6]),
		}
		replies = append(replies, reply)
	}
	return replies
}

func (m Montnets) GetBalance() (string, error) {
	param := m.assembleBalanceRequest(requestTypeReply)
	response, err := http.Get(m.BalanceEndpoint + param)
	if err != nil {
		logger.E("failed to query balance: %v\n", err)
		return "", ErrQueryBalanceFailed
	}
	if s := response.StatusCode; s != http.StatusOK {
		return "", ErrQueryBalanceFailed
	}
	balanceCount, err := m.handleBalanceResponse(response)
	if err != nil {
		logger.E("failed to query balance: %v\n", err)
		return "", ErrQueryBalanceFailed
	}

	return balanceCount, nil
}

func (m Montnets) assembleBalanceRequest(requestType string) string {
	form := url.Values{}
	form.Add(formKeyUserName, m.Username)
	form.Add(formKeyPassword, m.Password)
	return "?" + form.Encode()
}

func (m Montnets) handleBalanceResponse(response *http.Response) (string, error) {
	defer func() {
		_ = response.Body.Close()
	}()
	data, _ := ioutil.ReadAll(response.Body)
	var body string
	err := xml.Unmarshal(data, &body)
	if err != nil {
		return "", err
	}
	return body, nil
}

func (m Montnets) MultiXSend(contexts []*mo.SMSContext) ([]*mo.SMSContext, error) {
	//only send in production environment
	if !u.IsProduction() {
		logger.I("discard due to not in production environment!")
		return contexts, ErrNotInProduction
	}
	var succeedContexts []*mo.SMSContext
	phoneArray := m.extractPhoneArray(contexts)
	msgIDArray := m.extractMsgIDArray(contexts)
	contentArray := m.extractContentArray(contexts)

	pool := u.NewPool(poolSize, poolSize)
	defer pool.Release()
	jobCount := int(math.Ceil(float64(len(contexts)) / float64(maxSendNumEachTime))) // total job count
	pool.WaitCount(jobCount)
	logger.I("start sending multiX sms, phones: %v, length: %d,  total job count: %d", phoneArray, len(phoneArray), jobCount)
	for i := 0; i < jobCount; i++ {
		start := i * maxSendNumEachTime
		end := start + maxSendNumEachTime
		currentStep := i
		pool.JobQueue <- func() {
			defer func() {
				if r := recover(); r != nil {
					logger.E("err:%v\n", r)
				}
				pool.JobDone()
			}()
			if end > len(phoneArray) {
				end = len(phoneArray)
			}
			if start >= end {
				return
			}
			var response *http.Response
			var err error
			for i := 0; i < retryTimes; i++ {
				logger.D("start sending multiX sms, current step:%d, start:%d, end:%d, retryTimes:%d", currentStep, start, end, i)
				request := m.assembleMultiXSendRequest(msgIDArray[start:end], phoneArray[start:end], contentArray[start:end])
				response, err = http.PostForm(m.SendEndpoint, *request)
				if err != nil {
					logger.E("retryTimes:%d, failed to send multiX sms[%d:%d]:%v, %v\n", i, start, end, phoneArray[start:end], err)
					if i == retryTimes-1 {
						return
					}
					time.Sleep(time.Second)
				} else {
					break
				}
			}
			if s := response.StatusCode; s != http.StatusOK {
				return
			}
			err = m.handleSendResponse(response)
			if err != nil {
				logger.E("failed to handle multiX send response[%d:%d]: %v\n", start, end, err)
				return
			}
			succeedContexts = append(succeedContexts, contexts[start:end]...)
		}
	}
	pool.WaitAll()
	logger.I("finish sending multiX sms, total count: %d, succeed count: %d\n", len(contexts), len(succeedContexts))
	return succeedContexts, nil
}

func (m Montnets) assembleMultiXSendRequest(msgIDArray []string, phoneArray []string, contentArray []string) *url.Values {
	form := url.Values{}
	form.Add(formKeyUserName, m.Username)
	form.Add(formKeyPassword, m.Password)
	multixmt := make([]string, len(msgIDArray))
	for i := range msgIDArray {
		multixmt[i] = msgIDArray[i] + "|" + "*" + "|" + phoneArray[i] + "|" + base64.StdEncoding.EncodeToString([]byte(mahonia.NewEncoder("GBK").ConvertString(contentArray[i])))
	}
	form.Add(formMultixmt, strings.Join(multixmt, ","))
	return &form
}

func (m Montnets) extractMsgIDArray(contexts []*mo.SMSContext) []string {
	res := make([]string, len(contexts))
	for i := range contexts {
		res[i] = strconv.FormatInt(contexts[i].History.MsgID, 10)
	}
	return res
}

func (m Montnets) extractPhoneArray(contexts []*mo.SMSContext) []string {
	res := make([]string, len(contexts))
	for i := range contexts {
		res[i] = contexts[i].History.Phone
	}
	return res
}

func (m Montnets) extractContentArray(contexts []*mo.SMSContext) []string {
	res := make([]string, len(contexts))
	for i := range contexts {
		res[i] = contexts[i].History.Content
	}
	return res
}
