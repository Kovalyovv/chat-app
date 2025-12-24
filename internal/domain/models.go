package domain

import "time"

type Room struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	OwnerID    int64     `json:"ownerId"`
	InviteCode string    `json:"inviteCode"`
	CreatedAt  time.Time `json:"createdAt"`
}
