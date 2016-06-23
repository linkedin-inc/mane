package callback

import (
	"linkedin/service/mongodb"
	m "github.com/linkedin-inc/mane/model"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var GrowthStatisticsCallback = Name("growth_statistics")
var UpdateGrowthStatistics Callback = func(status *m.DeliveryStatus, history *m.SMSHistory) error {
	var code int32 = 3
	if status.StatusCode == 0 {
		code = 4
	}
	var err error
	mongodb.LogSessionExec("feedback", "growth_notice_history", func(c *mgo.Collection) error {
		err = c.Update(bson.M{
			"msg_id": status.MsgID,
			"phone":  status.Phone},
			bson.M{
				"$set": bson.M{
					"send_status": code,
				},
			})
		return err
	})
	return err
}

func RegisterAll() {
	Register(GrowthStatisticsCallback, UpdateGrowthStatistics)
}
