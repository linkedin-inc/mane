package service

import (
	"github.com/linkedin-inc/mane/logger"
	m "github.com/linkedin-inc/mane/model"
	v "github.com/linkedin-inc/mane/vendors"
)

func Pull(name v.Name) ([]*m.DeliveryStatus, []*m.Reply, error) {
	var statusList []*m.DeliveryStatus
	var replyList []*m.Reply

	vendors, err := v.GetByName(name)
	if err != nil {
		logger.E("occur error when find vendor %v : %v\n", name, err)
		return nil, nil, err
	}
	for _, vendor := range vendors {
		for {
			statuses, err := fetchStatus(vendor)
			if err != nil {
				logger.E("failed to pull status from %v : %v\n", vendor.Name(), err)
				break
			}
			if len(statuses) == 0 {
				break
			}
			statusList = append(statusList, statuses...)
		}
		for {
			replies, err := fetchReply(vendor)
			if err != nil {
				logger.E("failed to pull reply from %v : %v\n", vendor.Name(), err)
				break
			}
			if len(replies) == 0 {
				break
			}
			replyList = append(replyList, replies...)
		}
	}
	return statusList, replyList, nil
}

func fetchStatus(vendor v.Vendor) ([]*m.DeliveryStatus, error) {
	statuses, err := vendor.Status()
	if err != nil {
		return nil, err
	}
	return statuses, nil
}

func fetchReply(vendor v.Vendor) ([]*m.Reply, error) {
	replies, err := vendor.Reply()
	if err != nil {
		return nil, err
	}
	return replies, nil
}
