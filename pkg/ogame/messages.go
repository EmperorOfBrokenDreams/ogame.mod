package ogame

import "time"

// Message ...
type Message struct {
	ID        int64
	Title     string
	Content   string
	CreatedAt time.Time
}
