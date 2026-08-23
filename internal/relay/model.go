package relay

import "time"

type Notification struct {
	ID        string
	Kind      string
	Actor     string
	Content   string
	TargetURL string
	CreatedAt time.Time
}

type NotificationPage struct {
	Notifications []Notification
	HasNext       bool
}
