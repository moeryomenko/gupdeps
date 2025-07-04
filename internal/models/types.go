package models

import "time"

// Dependency represents a Go module dependency
type Dependency struct {
	Name           string `json:"name"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	UpdateNeeded   bool   `json:"update_needed"`
}

// CommitInfo represents commit information
type CommitInfo struct {
	Hash    string
	Message string
	Date    time.Time
}

// UpdateAnalysis represents the analysis result for an update
type UpdateAnalysis struct {
	Dependency      *Dependency
	Commits         []CommitInfo
	ShouldUpdate    bool
	UpdateReason    string
	RejectionReason string
}
