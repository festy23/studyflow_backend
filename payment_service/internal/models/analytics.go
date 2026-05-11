package models

import "time"

type TutorAnalytics struct {
	TutorID               string
	From                  *time.Time
	To                    *time.Time
	TotalRevenueRub       int64
	CompletedLessonsCount int64
	CancelledLessonsCount int64
	ActiveStudentsCount   int64
	UnpaidLessonsCount    int64
}
