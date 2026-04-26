package web

// JobListItem augments a Job for templates (pause is tracked in-memory, not the DB).
type JobListItem struct {
	Job
	IsPaused bool
}

// StatusClass is used for CSS: .status-paused when pause is active.
func (j JobListItem) StatusClass() string {
	if j.IsPaused && j.Status == StatusWorking {
		return "paused"
	}

	return j.Status
}

// StatusLabel is the user-visible status (shows "paused" over "working" when set).
func (j JobListItem) StatusLabel() string {
	if j.IsPaused && j.Status == StatusWorking {
		return "paused"
	}

	return j.Status
}
