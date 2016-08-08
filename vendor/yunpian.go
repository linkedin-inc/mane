package vendor

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/linkedin-inc/mane/logger"
	m "github.com/linkedin-inc/mane/model"
	"github.com/linkedin-inc/mane/util"
)

const (
	formKeyAPIKey   = "apikey"
	formKeyMobile   = "mobile"
	formKeyText     = "text"
	formKeyUID      = "uid"
	formKeyPageSize = "page_size"
)

var (
	NameYunpian = Name("yunpian")
)

type Yunpian struct {
	APIKey            string
	SendEndpoint      string
	MultiSendEndpoint string
	StatusEndpoint    string
	ReplyEndpoint     string
}

type yunpianSendResponse struct {
	Code   int    `json:"code"`
	Msg    string `json:"msg"`
	Result string `json:"result"`
}

type yunpianStatusResponse struct {
	Code      int32     `json:"code"`
	Msg       string    `json:"msg"`
	SMSStatus []*status `json:"msg_status"`
}

type status struct {
	SID             int64  `json:"sid"`
	UID             string `json:"uid"`
	UserReceiveTime string `json:"user_receive_time"`
	ErrorMsg        string `json:"error_msg"`
	Mobile          string `json:"mobile"`
	ReportStatus    string `json:"report_status"`
}

type replyResponse struct {
	Code     int32    `json:"code"`
	Msg      string   `json:"msg"`
	SMSReply []*reply `json:"sms_reply"`
}

type reply struct {
	Mobile     string `json:"mobile"`
	ReplyTime  string `json:"reply_time"`
	Text       string `json:"text"`
	Extend     string `json:"extend"`
	BaseExtend string `json:"base_extend"`
}

func NewYunpian(apiKey, sendEndpoint, multiSendEndpoint, statusEndpoint, replyEndpoint string) Yunpian {
	return Yunpian{
		APIKey:            apiKey,
		SendEndpoint:      sendEndpoint,
		MultiSendEndpoint: multiSendEndpoint,
		StatusEndpoint:    statusEndpoint,
		ReplyEndpoint:     replyEndpoint,
	}
}

func (y Yunpian) Name() Name {
	return NameYunpian
}

func (y Yunpian) Send(seqID string, phoneArray []string, contentArray []string) error {
	//only send in production environment
	if !util.IsProduction() {
		logger.I("discard due to not in production environment!")
		return ErrNotInProduction
	}
	form := y.assembleSendRequest(seqID, phoneArray, contentArray)
	endpoint := y.SendEndpoint
	if len(contentArray) > 1 {
		endpoint = y.MultiSendEndpoint
	}
	request, _ := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
	request.Header.Add("Accept", "application/json;charset=utf-8;")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded;charset=utf-8;")
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		logger.E("failed to send sms: %v\n", err)
		return err
	}
	if s := response.StatusCode; s != http.StatusOK {
		return ErrSendSMSFailed
	}
	err = y.handleSendResponse(response)
	if err != nil {
		return err
	}
	return nil
}

func (y Yunpian) assembleSendRequest(seqID string, phoneArray []string, contentArray []string) *url.Values {
	form := url.Values{}
	form.Add(formKeyAPIKey, y.APIKey)
	form.Add(formKeyMobile, strings.Join(phoneArray, ","))
	form.Add(formKeyText, strings.Join(contentArray, ","))
	form.Add(formKeyUID, seqID)
	return &form
}

func (y Yunpian) handleSendResponse(response *http.Response) error {
	defer func() {
		_ = response.Body.Close()
	}()
	data, _ := ioutil.ReadAll(response.Body)
	var body yunpianSendResponse
	err := json.Unmarshal(data, &body)
	if err != nil {
		logger.E("occur error when handle send response: %v\n", err)
		return err
	}
	if body.Code != 0 {
		logger.E("send failed, %s, %s", body.Msg, body.Result)
		return ErrSendSMSFailed
	}
	return nil
}
func (y Yunpian) Status() ([]*m.DeliveryStatus, error) {
	form := y.assemblePullRequest()
	request, _ := http.NewRequest("POST", y.StatusEndpoint, strings.NewReader(form.Encode()))
	request.Header.Add("Accept", "application/json;charset=utf-8;")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded;charset=utf-8;")
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		logger.E("failed to check status: %v\n", err)
		return nil, ErrGetStatusFailed
	}
	if s := response.StatusCode; s != http.StatusOK {
		logger.E("failed to check status: %d\n", s)
		return nil, ErrGetStatusFailed
	}
	status, err := y.handleStatusResponse(response)
	if err != nil {
		logger.E("failed to handle status response: %v\n", err)
		return nil, ErrGetStatusFailed
	}
	var parsedStatus []*m.DeliveryStatus
	if len(status) == 0 {
		return parsedStatus, nil
	}
	parsedStatus = y.parseStatus(status)
	return parsedStatus, nil
}

func (y Yunpian) assemblePullRequest() *url.Values {
	form := url.Values{}
	form.Add(formKeyAPIKey, y.APIKey)
	form.Add(formKeyPageSize, "100")
	return &form
}

func (y Yunpian) handleStatusResponse(response *http.Response) ([]*status, error) {
	defer func() {
		if err := response.Body.Close(); err != nil {
			logger.I("Close error %v\n", err)
		}
	}()
	data, _ := ioutil.ReadAll(response.Body)
	var body yunpianStatusResponse
	err := json.Unmarshal(data, &body)
	if err != nil {
		logger.E("occur error when handle status response: %v\n", err)
		return nil, err
	}
	if body.Code != 0 {
		logger.E("check status failed, %s", body.Msg)
		return nil, errors.New(body.Msg)
	}
	return body.SMSStatus, nil
}

func (y Yunpian) parseStatus(raw []*status) []*m.DeliveryStatus {
	var statuses []*m.DeliveryStatus
	for _, aRawRecord := range raw {
		timestamp, err := time.ParseInLocation("2006-01-02 15:04:05", aRawRecord.UserReceiveTime, time.Local)
		if err != nil {
			logger.E("failed to parse time: %v\n", err)
			//discard and go ahead
			continue
		}
		status := &m.DeliveryStatus{
			MsgID:      util.Atoi64Safe(aRawRecord.UID, -1),
			Timestamp:  timestamp,
			Phone:      aRawRecord.Mobile,
			StatusCode: 0,
		}
		//omit fixed detail msg like '"SUCCESS"'
		if aRawRecord.ReportStatus != "SUCCESS" {
			status.StatusCode = 1
			status.ErrorMsg = aRawRecord.ErrorMsg
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func (y Yunpian) Reply() ([]*m.Reply, error) {
	form := y.assemblePullRequest()
	request, _ := http.NewRequest("POST", y.ReplyEndpoint, strings.NewReader(form.Encode()))
	request.Header.Add("Accept", "application/json;charset=utf-8;")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded;charset=utf-8;")
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		logger.E("failed to check status: %v\n", err)
		return nil, ErrGetReplyFailed
	}
	if s := response.StatusCode; s != http.StatusOK {
		logger.E("failed to check status: %d\n", s)
		return nil, ErrGetReplyFailed
	}
	replies, err := y.handleReplyResponse(response)
	if err != nil {
		logger.E("failed to handle status response: %v\n", err)
		return nil, ErrGetReplyFailed
	}
	var parsedReplies []*m.Reply
	if len(replies) == 0 {
		return parsedReplies, nil
	}
	parsedReplies = y.parseReply(replies)
	return parsedReplies, nil
}

func (y Yunpian) handleReplyResponse(response *http.Response) ([]*reply, error) {
	defer func() {
		_ = response.Body.Close()
	}()
	data, _ := ioutil.ReadAll(response.Body)
	var body replyResponse
	err := json.Unmarshal(data, &body)
	if err != nil {
		logger.E("occur error when handle reply response: %v\n", err)
		return nil, err
	}
	if body.Code != 0 {
		logger.E("pull reply failed, %s", body.Msg)
		return nil, errors.New(body.Msg)
	}
	return body.SMSReply, nil
}

func (y Yunpian) parseReply(raw []*reply) []*m.Reply {
	var replies []*m.Reply
	for _, aRawRecord := range raw {
		timestamp, err := time.ParseInLocation("2006-01-02 15:04:05", aRawRecord.ReplyTime, time.Local)
		if err != nil {
			logger.E("failed to parse time: %v\n", err)
			//discard and go ahead
			continue
		}
		reply := &m.Reply{
			Timestamp: timestamp,
			Phone:     aRawRecord.Mobile,
			Msg:       strings.TrimSpace(aRawRecord.Text),
		}
		replies = append(replies, reply)
	}
	return replies
}

func (y Yunpian) GetBalance() (string, error) {
	return "", errors.New("not implemented")
}

func (y Yunpian) MultiXSend(msgIDArray []string, phoneArray []string, contentArray []string) error {
	return errors.New("not implemented")
}
