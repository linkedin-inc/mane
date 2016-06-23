package track

import "fmt"

const (
	trackID = "track_id"
	userID  = "user_id"
)

type TrackableLink struct {
	TrackID string
	UserID  int64
	Append  bool
}

func NewTrackableLink(userID int64, trackID string) TrackableLink {
	return TrackableLink{
		TrackID: trackID,
		UserID:  userID,
		Append:  false,
	}
}

func (l TrackableLink) String() string {
	if l.Append {
		//append mode
		return fmt.Sprintf("&"+userID+"=%d"+"&"+trackID+"=%s", l.UserID, l.TrackID)
	}
	return fmt.Sprintf("?"+userID+"=%d"+"&"+trackID+"=%s", l.UserID, l.TrackID)
}

//GenerateTrackableLink return a trackable uri for given userID and trackID
