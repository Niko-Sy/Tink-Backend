package models

import "time"

type MuteRecord struct {
	recordId  string
	memberId  string
	roomId    string
	mutedBy   string
	muteStart time.Time
	muteEnd   time.Time
	reason    string
	active    bool
}
