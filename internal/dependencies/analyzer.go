package dependencies

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/moeryomenko/gupdeps/internal/models"
	"github.com/moeryomenko/gupdeps/internal/utils"
)

// CommitAnalyzer analyzes commits to determine if updates should be applied
type CommitAnalyzer struct {
	logger *utils.Logger
}

// NewCommitAnalyzer creates a new commit analyzer
func NewCommitAnalyzer(logger *utils.Logger) *CommitAnalyzer {
	return &CommitAnalyzer{
		logger: logger,
	}
}

// AnalyzeCommits analyzes commit messages to determine if update should be applied
func (ca *CommitAnalyzer) AnalyzeCommits(commits []models.CommitInfo) (shouldUpdate bool, reason, riskLevel string) {
	patterns := map[string]*regexp.Regexp{
		"fix":     regexp.MustCompile(`(?i)(fix|bug|patch|resolve|correct)`),
		"perf":    regexp.MustCompile(`(?i)(perf|optimize|performance|speed|faster)`),
		"break":   regexp.MustCompile(`(?i)(breaking|break|remove|deprecate|BREAKING)`),
		"feature": regexp.MustCompile(`(?i)(feat|feature|add|new)`),
	}

	counts := map[string]int{
		"fix":     0,
		"perf":    0,
		"break":   0,
		"feature": 0,
	}

	for _, commit := range commits {
		msg := commit.Message
		for category, pattern := range patterns {
			if pattern.MatchString(msg) {
				counts[category]++
			}
		}
	}

	// Decision logic
	if counts["break"] > 0 {
		return false, "", ca.formatRejectionReason(counts["break"])
	}

	if counts["fix"] > 0 || counts["perf"] > 0 {
		return true, ca.formatApprovalReason(counts), ""
	}

	if counts["feature"] > 0 {
		return true, ca.formatFeatureReason(counts["feature"]), ""
	}

	return false, "", "No significant improvements found"
}

// formatRejectionReason creates a rejection message
func (ca *CommitAnalyzer) formatRejectionReason(breakingChanges int) string {
	return fmt.Sprintf("Contains %d breaking changes", breakingChanges)
}

// formatApprovalReason creates an approval message based on fixes and optimizations
func (ca *CommitAnalyzer) formatApprovalReason(counts map[string]int) string {
	var reasons []string
	if counts["fix"] > 0 {
		reasons = append(reasons, fmt.Sprintf("%d fixes", counts["fix"]))
	}
	if counts["perf"] > 0 {
		reasons = append(reasons, fmt.Sprintf("%d optimizations", counts["perf"]))
	}
	return strings.Join(reasons, ", ")
}

// formatFeatureReason creates a feature-based approval message
func (ca *CommitAnalyzer) formatFeatureReason(featureCount int) string {
	return fmt.Sprintf("%d new features", featureCount)
}

// AnalyzeUpdate performs complete analysis for a dependency update
func (ca *CommitAnalyzer) AnalyzeUpdate(dep *models.Dependency, commits []models.CommitInfo) *models.UpdateAnalysis {
	shouldUpdate, reason, rejection := ca.AnalyzeCommits(commits)

	return &models.UpdateAnalysis{
		Dependency:      dep,
		Commits:         commits,
		ShouldUpdate:    shouldUpdate,
		UpdateReason:    reason,
		RejectionReason: rejection,
	}
}
