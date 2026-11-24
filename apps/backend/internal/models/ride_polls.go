package models

import "time"

const RidePollTimezone = "Asia/Almaty"

type RidePollInput struct {
	City        string      `json:"city"`
	Description string      `json:"description"`
	Options     []time.Time `json:"options"`
}

type RidePoll struct {
	ID               string           `json:"id"`
	City             string           `json:"city"`
	Description      string           `json:"description"`
	Timezone         string           `json:"timezone"`
	Options          []RidePollOption `json:"options"`
	SelectedOptionID string           `json:"selected_option_id,omitempty"`
	MyOptionIDs      []string         `json:"my_option_ids"`
	CanManage        bool             `json:"can_manage"`
	OrganizerID      string           `json:"-"`
	CreatedAt        time.Time        `json:"-"`
}

type RidePollOption struct {
	ID       string    `json:"id"`
	StartsAt time.Time `json:"starts_at"`
	Votes    int64     `json:"votes"`
}
