package sms

import (
	"linkedin/avro"
	"linkedin/bi"
	"linkedin/config"
	"linkedin/log"
	"linkedin/model/share"
	p_profile "linkedin/proto/profile"
	"linkedin/service/shortlink"
	s "linkedin/service/sms/service"
	t "linkedin/service/sms/template"
	"linkedin/util"
	"net/url"
	"strconv"
	"strings"

	p_avro "github.com/elodina/go-avro"
)

var (
	InviteUserTemplate            = t.Name("invite_user")              //"恭喜你获得{name}注册邀请码{code},快来发现行业活动和职场牛人吧!点击{download} ."
	AskLoginTemplate              = t.Name("ask_login_invitation")     //"亲爱的{name},{user}提醒您登录赤兔,与TA一起赢取赤兔#疯狂星期五#活动奖品,点击链接 {link} 下载赤兔,嗨翻职场!"
	VerificationCodeTemplate      = t.Name("verification_code")        //"{name}注册验证码: {code},仅用于注册验证,请勿告知他人。{service}"
	ChangePasswordTemplate        = t.Name("change_password")          //"{name}忘记密码验证码: {code},仅用于忘记密码验证,请勿告知他人。{service}"
	RandomPasswordOnlineTemplate  = t.Name("random_password_online")   //"报名还差一步就成功了，请下载并用手机号登录赤兔。您的赤兔密码为{pwd}，点击下载赤兔{url}"
	RandomPasswordOfflineTemplate = t.Name("random_password_offline")  //"报名已提交，请尽快下载赤兔，更有可能通过审核。请用手机号登录，您的赤兔密码为{pwd}，点击下载赤兔：{url}"
	ZhimaRegistrationTemplate     = t.Name("zhima_registration")       //"注册成功，请下载使用赤兔app,并完善个人资料即可提高芝麻信用, {url}"
	ZhimaRegistration1Template    = t.Name("zhima_registration_1")     //"注册成功。到赤兔完善资料、建立人脉将有效提升您的职场信用 {url} "
	ZhimaRegistration2Template    = t.Name("zhima_registration_2")     //"注册成功。为提升您的职场信用，请尽快登录赤兔完善资料建立人脉 {url} "
	ContactRegistrationTemplate   = t.Name("contact_reg_notification") //"亲爱的{toName}，您的通讯录好友{fromName}{companyAndTitle}加入了赤兔。点击链接，和Ta打个招呼吧！{link} 回TD退订"
	ContactMsgTemplate            = t.Name("contact_msg")              //"哈啰！我是{company}{name},我已经在赤兔累积了{friendcount}位人脉,参加了{group}{activity}等线上线下活动,快来赤兔跟我一起感受真实有趣的职场:{link} 回TD退订"

	RecallCategory                = t.Category("recall")
	GatheringPromotionCategory    = t.Category("gathering_promotion")
	GatheringNotificationCategory = t.Category("gathering_notification")
	ContactCategory               = t.Category("contact")
	OpsAlertCategory              = t.Category("ops_alert")
	InviteAllCategory             = t.Category("invite_all")
	ZhimaRecallCategory           = t.Category("zhima_recall")
	AutoRecallCategory            = t.Category("auto_recall")
	PushV2RecallCategory          = t.Category("crm_push_v2")
)

func PushToBI(phone, content string, smsType *p_avro.GenericEnum) {
	record := &avro.SmsSent{
		Timestamp:   util.CurrentTimeMillis(),
		TgtPhone:    phone,
		SmsContent:  content,
		IsDelivered: false,
		SmsSubtype:  0,
		SmsType:     smsType,
	}
	bi.SaveAvro(record)
}

func SendVerificationCode(phone, verificationCode string) error {
	variables := map[string]string{
		"code":    verificationCode,
		"name":    AppName,
		"service": ContactUs,
	}
	log.Info.Printf("variables: %v\n", variables)
	_, content, err := s.Send(VerificationCodeTemplate, variables, []string{phone})
	if err != nil {
		log.Error.Printf("failed to send verification code: %v\n", err)
		return err
	}
	PushToBI(phone, content, avro.PhoneMsgType_VERIFY)
	return nil
}

func SendResetPasswordCode(phone, code string) error {
	variables := map[string]string{
		"code":    code,
		"name":    AppName,
		"service": ContactUs,
	}
	log.Info.Printf("variables: %v\n", variables)
	_, content, err := s.Send(ChangePasswordTemplate, variables, []string{phone})
	if err != nil {
		log.Error.Printf("failed to send reset password code: %v\n", err)
		return err
	}
	PushToBI(phone, content, avro.PhoneMsgType_VERIFY)
	return nil
}

func SendInvitation(phone, code string) error {
	variables := map[string]string{
		"code":     code,
		"name":     AppName,
		"download": ContactUs,
	}
	log.Info.Printf("variables: %v\n", variables)
	_, content, err := s.Send(InviteUserTemplate, variables, []string{phone})
	if err != nil && err != s.ErrNotAllowed {
		log.Error.Printf("failed to send invitation: %v\n", err)
		return err
	}
	PushToBI(phone, content, avro.PhoneMsgType_INVITE)
	return nil
}

func SendInvitations(phones []string, data map[string]string) error {
	for _, phone := range phones {
		variables := map[string]string{
			"name":        data["name"],
			"company":     data["company"],
			"friendcount": data["friendcount"],
			"group":       data["group"],
			"activity":    data["activity"],
			"link":        data["link"],
		}
		log.Info.Printf("variables: %v\n", variables)
		_, content, err := s.Send(ContactMsgTemplate, variables, []string{phone})
		if err != nil && err != s.ErrNotAllowed {
			log.Error.Printf("failed to send invitations: %v\n", err)
			continue
		}
		PushToBI(phone, content, avro.PhoneMsgType_INVITE)
	}
	return nil
}

func SendRandomPassword(name, phone, password, url string, isOnline bool) error {
	variables := map[string]string{
		"name": name,
		"pwd":  password,
		"url":  url,
	}
	log.Info.Printf("variables: %v\n", variables)
	var err error
	var content string
	if isOnline {
		_, content, err = s.Send(RandomPasswordOnlineTemplate, variables, []string{phone})
	} else {
		_, content, err = s.Send(RandomPasswordOfflineTemplate, variables, []string{phone})
	}
	if err != nil && err != s.ErrNotAllowed {
		log.Error.Printf("failed to send random password: %v\n", err)
		return err
	}
	PushToBI(phone, content, avro.PhoneMsgType_EVENT_NOTICE)
	return nil
}

func SendLoginInvitation(phone, name, targe, url string) error {
	variables := map[string]string{
		"name": targe,
		"user": name,
		"link": url,
	}
	log.Info.Printf("variables: %v\n", variables)
	_, content, err := s.Send(AskLoginTemplate, variables, []string{phone})
	if err != nil && err != s.ErrNotAllowed {
		log.Error.Printf("failed to send login invitation: %v\n", err)
		return err
	}
	PushToBI(phone, content, avro.PhoneMsgType_INVITE)
	return nil
}

func SendContactInvitation(phone string, data map[string]string) (string, error) {
	variables := map[string]string{
		"name":        data["name"],
		"company":     data["company"],
		"friendcount": data["friendcount"],
		"group":       data["group"],
		"activity":    data["activity"],
		"link":        data["link"],
	}
	log.Info.Printf("variables: %v\n", variables)
	_, content, err := s.Send(ContactMsgTemplate, variables, []string{phone})
	if err != nil && err != s.ErrNotAllowed {
		log.Error.Printf("failed to send contact invitation: %v\n", err)
		return "", err
	}
	PushToBI(phone, content, avro.PhoneMsgType_OTHER)
	return content, nil
}

func SendContactRegistration(toPhones []string, toNames []string, fromUser *p_profile.Profile) error {
	fromName := fromUser.GetName()
	fromCompany := fromUser.GetCompanyname()
	fromTitle := fromUser.GetTitlename()
	companyAndTitle := ""
	if fromCompany != "" && fromTitle != "" {
		companyAndTitle = "(" + fromCompany + " " + fromTitle + ")"
	} else if fromCompany != "" {
		companyAndTitle = "(" + fromCompany + ")"
	} else if fromTitle != "" {
		companyAndTitle = "(" + fromTitle + ")"
	}
	fromUserIDHash := share.GetHash(&share.Share{UserID: fromUser.GetXId()})
	profileCtURL := strings.Replace(CTURL, "{ctFromUserID}", strconv.FormatInt(fromUser.GetXId(), 10), 1)
	profileFallBackURL := strings.Replace(FallbackURL, "{fromUserID}", fromUserIDHash, 2)
	profileURL := CTURLPrefix + url.QueryEscape(profileCtURL) + CTURLParamKey + url.QueryEscape(profileFallBackURL)
	profileURL, shortLinkErr := shortlink.ShortLink(profileURL)
	if shortLinkErr != nil {
		return shortLinkErr
	}
	if util.IsProduction() {
		profileURL = "http://" + config.ShortLinkDomain() + "/" + profileURL
	} else {
		profileURL = config.ShortLinkServerURL() + "/" + profileURL
	}
	for i := 0; i < len(toPhones); i++ {
		variables := map[string]string{
			"toName":          toNames[i],
			"fromName":        fromName,
			"companyAndTitle": companyAndTitle,
			"link":            profileURL,
		}
		log.Info.Printf("variables: %v\n", variables)
		phones := []string{toPhones[i]}
		_, _, err := s.Send(ContactRegistrationTemplate, variables, phones)
		if err != nil && err != s.ErrNotAllowed {
			log.Error.Printf("failed to send contact registration notification: %v\n", err)
			continue
		}
	}
	return nil
}

func SendZhimaUpdateProfilePromotion(phone, url string) error {
	variables := map[string]string{
		"url": url,
	}
	log.Info.Printf("variables: %v\n", variables)
	_, content, err := s.Send(ZhimaRegistrationTemplate, variables, []string{phone})
	if err != nil && err != s.ErrNotAllowed {
		log.Error.Printf("failed to send promotion to zhima user: %v\n", err)
		return err
	}
	PushToBI(phone, content, avro.PhoneMsgType_ZHIMA)
	return nil
}

func SendZhimaRegistrationPromotion(userID int64, phone, url string) error {
	variables := map[string]string{
		"url": url,
	}
	log.Info.Printf("variables: %v\n", variables)
	choose := userID % int64(2)
	var err error
	var content string
	if choose == int64(0) {
		_, content, err = s.Send(ZhimaRegistration1Template, variables, []string{phone})
	} else if choose == int64(1) {
		_, content, err = s.Send(ZhimaRegistration2Template, variables, []string{phone})
	}
	if err != nil && err != s.ErrNotAllowed {
		log.Error.Printf("failed to send promotion to zhima user: %v\n", err)
		return err
	}
	PushToBI(phone, content, avro.PhoneMsgType_ZHIMA)
	return nil
}

func SendZhimaRecall(phone, content string) error {
	_, err := s.Push(t.MarketingChannel, ZhimaRecallCategory, content, []string{phone})
	if err != nil {
		log.Error.Printf("failed to send zhima recall message: %v\n", err)
		return err
	}
	return nil
}

func SendRecallMessage(phone, content string, category ...t.Category) (int64, error) {
	var msgID string
	var err error
	if len(category) > 0 {
		msgID, err = s.Push(t.MarketingChannel, category[0], content, []string{phone})
	} else {
		msgID, err = s.Push(t.MarketingChannel, RecallCategory, content, []string{phone})
	}
	if err != nil {
		log.Error.Printf("failed to send recall message: %v\n", err)
		return 0, err
	}
	i, _ := strconv.ParseInt(msgID, 10, 64)
	PushToBI(phone, content, avro.PhoneMsgType_RECALL)
	return i, nil
}

func SendGatheringPromotion(phone, content string) (int64, error) {
	msgID, err := s.Push(t.MarketingChannel, GatheringPromotionCategory, content, []string{phone})
	if err != nil {
		log.Error.Printf("failed to send gathering promotion: %v\n", err)
		return 0, err
	}
	i, _ := strconv.ParseInt(msgID, 10, 64)
	PushToBI(phone, content, avro.PhoneMsgType_EVENT_NOTICE)
	return i, nil
}

func SendGatheringNotification(phone []string, content string, specifiedChannel string) (int64, error) {
	c := t.MarketingChannel
	if specifiedChannel == "production" {
		c = t.ProductionChannel
	}
	msgID, err := s.Push(c, GatheringNotificationCategory, content, phone)
	if err != nil {
		log.Error.Printf("failed to send gathering notification: %v\n", err)
		return 0, err
	}
	i, _ := strconv.ParseInt(msgID, 10, 64)
	return i, nil
}

func SendContactMessage(phone, content string) error {
	_, err := s.Push(t.MarketingChannel, ContactCategory, content, []string{phone})
	if err != nil {
		log.Error.Printf("failed to send contact custom message: %v\n", err)
		return err
	}
	PushToBI(phone, content, avro.PhoneMsgType_INVITE)
	return nil
}

func SendOpsAlert(phone, content string) error {
	_, err := s.Push(t.MarketingChannel, OpsAlertCategory, content, []string{phone})
	if err != nil {
		log.Error.Printf("failed to send ops alert: %v\n", err)
		return err
	}
	//FIXME push to BI?
	//PushToBI(phone,content,nil)
	return nil
}

// crm push v2 sms msg
func SendCRMPushV2(phone, content string, channel t.Channel) (int64, error) {
	var msgID string
	var err error
	msgID, err = s.Push(channel, PushV2RecallCategory, content, []string{phone})
	if err != nil {
		log.Error.Printf("failed to send recall message: %v\n", err)
		return 0, err
	}
	i, _ := strconv.ParseInt(msgID, 10, 64)
	PushToBI(phone, content, avro.PhoneMsgType_CRM_PUSH_V2)
	return i, nil
}
