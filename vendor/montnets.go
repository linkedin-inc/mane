package vendor

import (
	"encoding/xml"
	"errors"
	"io/ioutil"
	"linkedin/log"
	"linkedin/util"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	mo "github.com/linkedin-inc/mane/model"
	t "github.com/linkedin-inc/mane/template"
)

const (
	formKeyMsgID        = "MsgId"
	formKeyUserName     = "userId"
	formKeyPassword     = "password"
	formKeyPhoneArray   = "pszMobis"
	formKeyMessageArray = "pszMsg"
	formKeyPhoneCount   = "iMobiCount"
	formKeySubPort      = "pszSubPort"
	formKeyRequestType  = "iReqType"
	requestTypeReply    = "1"
	requestTypeStatus   = "2"
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
	StatusEndpoint  string
	BalanceEndpoint string
}

func NewMontnets(username, password, sendEndpoint, statusEndpoint, balanceEndpoint string) Montnets {
	return Montnets{
		Username:        username,
		Password:        password,
		SendEndpoint:    sendEndpoint,
		StatusEndpoint:  statusEndpoint,
		BalanceEndpoint: balanceEndpoint,
	}
}

func (m Montnets) Name() Name {
	return NameMontnets
}

func (m Montnets) Register(channel t.Channel) {
	if Registry.Channels == nil {
		Registry.Channels = make(map[t.Channel][]Vendor)
	}
	vendors, existed := Registry.Channels[channel]
	if !existed {
		Registry.Channels[channel] = []Vendor{m}
	} else {
		Registry.Channels[channel] = append(vendors, m)
	}
	if Registry.Vendors == nil {
		Registry.Vendors = make(map[Name][]Vendor)
	}
	vendors, existed = Registry.Vendors[NameMontnets]
	if !existed {
		Registry.Vendors[NameMontnets] = []Vendor{m}
	} else {
		Registry.Vendors[NameMontnets] = append(vendors, m)
	}
}

//Send sms to given phone number with content
func (m Montnets) Send(seqID string, phoneArray []string, contentArray []string) error {
	//only send in production environment
	if !util.IsProduction() {
		log.Info.Printf("discard due to not in production environment!")
		return ErrNotInProduction
	}
	request := m.assembleSendRequest(seqID, phoneArray, contentArray)
	response, err := http.PostForm(m.SendEndpoint, *request)
	if err != nil {
		log.Error.Printf("failed to send sms: %v\n", err)
		return err
	}
	if s := response.StatusCode; s != http.StatusOK {
		return ErrSendSMSFailed
	}
	err = m.handleSendResponse(response)
	if err != nil {
		log.Error.Printf("failed to handle send response: %v\n", err)
		return err
	}
	return nil
}

func (m Montnets) assembleSendRequest(seqID string, phoneArray []string, contentArray []string) *url.Values {
	form := url.Values{}
	form.Add(formKeyUserName, m.Username)
	form.Add(formKeyPassword, m.Password)
	form.Add(formKeyPhoneArray, strings.Join(phoneArray, ","))
	form.Add(formKeyMessageArray, strings.Join(contentArray, ","))
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

func (m Montnets) Status() ([]mo.DeliveryStatus, error) {
	request := m.assembleUpstreamRequest(requestTypeStatus)
	response, err := http.PostForm(m.StatusEndpoint, *request)
	if err != nil {
		log.Error.Printf("failed to check status: %v\n", err)
		return nil, ErrGetStatusFailed
	}
	if s := response.StatusCode; s != http.StatusOK {
		return nil, ErrGetStatusFailed
	}
	status, err := m.handleUpstreamResponse(response)
	if err != nil {
		log.Error.Printf("failed to handle status response: %v\n", err)
		return nil, ErrGetStatusFailed
	}
	var parsedStatus []mo.DeliveryStatus
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

func (m Montnets) parseStatus(raw []string) []mo.DeliveryStatus {
	var statuses []mo.DeliveryStatus
	for _, rawRecord := range raw {
		splited := strings.Split(rawRecord, ",")
		timestamp, err := time.ParseInLocation("2006-01-02 15:04:05", splited[1], time.Local)
		if err != nil {
			log.Error.Printf("failed to parse time: %v\n", err)
			//discard and go ahead
			continue
		}
		status := mo.DeliveryStatus{
			MsgID:      util.Atoi64(splited[5]),
			Timestamp:  timestamp,
			Phone:      splited[4],
			StatusCode: util.Atoi32(splited[7]),
		}
		//omit fixed detail msg like 'DELIVERED'
		if splited[7] != "0" {
			status.ErrorMsg = splited[8]
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func (m Montnets) Reply() ([]mo.Reply, error) {
	request := m.assembleUpstreamRequest(requestTypeReply)
	response, err := http.PostForm(m.StatusEndpoint, *request)
	if err != nil {
		log.Error.Printf("failed to get reply: %v\n", err)
		return nil, ErrGetReplyFailed
	}
	if s := response.StatusCode; s != http.StatusOK {
		return nil, ErrGetReplyFailed
	}
	replies, err := m.handleUpstreamResponse(response)
	if err != nil {
		log.Error.Printf("failed to handle reply response: %v\n", err)
		return nil, ErrGetReplyFailed
	}
	var parsedReplies []mo.Reply
	if len(replies) == 0 {
		return parsedReplies, nil
	}
	parsedReplies = m.parseReply(replies)
	return parsedReplies, nil
}

func (m Montnets) parseReply(raw []string) []mo.Reply {
	var replies []mo.Reply
	for _, rawRecord := range raw {
		splited := strings.Split(rawRecord, ",")
		timestamp, err := time.ParseInLocation("2006-01-02 15:04:05", splited[1], time.Local)
		if err != nil {
			log.Error.Printf("failed to parse time: %v\n", err)
			//discard and go ahead
			continue
		}
		reply := mo.Reply{
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
	log.Debug.Println("m.BalanceEndpoint + param", m.BalanceEndpoint+param)
	response, err := http.Get(m.BalanceEndpoint + param)
	if err != nil {
		log.Error.Printf("failed to query balance: %v\n", err)
		return "", ErrQueryBalanceFailed
	}
	if s := response.StatusCode; s != http.StatusOK {
		return "", ErrQueryBalanceFailed
	}
	balanceCount, err := m.handleBalanceResponse(response)
	if err != nil {
		log.Error.Printf("failed to query balance: %v\n", err)
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
