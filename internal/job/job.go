package job

import "time"

type PrintJob struct {
	ID          string
	Raw         []byte
	ContentType string
	Source      string
	CreatedAt   time.Time
}
