package dependencies

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/moeryomenko/gupdeps/internal/models"
	"github.com/moeryomenko/gupdeps/internal/utils"
)

// GitOperations handles Git-related operations
type GitOperations struct {
	logger *utils.Logger
}

// NewGitOperations creates a new GitOperations instance
func NewGitOperations(logger *utils.Logger) *GitOperations {
	return &GitOperations{
		logger: logger,
	}
}

// GetCommitsBetweenVersions fetches commits between two versions
func (g *GitOperations) GetCommitsBetweenVersions(dep *models.Dependency) ([]models.CommitInfo, error) {
	if dep.CurrentVersion == dep.LatestVersion {
		return []models.CommitInfo{}, nil
	}

	// Create a temporary directory for the repository
	tempDir, err := os.MkdirTemp("", "dependency-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up when done

	repoURL := g.determineRepositoryURL(dep.Name)

	// Clone the repository with minimal depth
	if err := g.cloneRepository(repoURL, tempDir); err != nil {
		return nil, err
	}

	// Try to fetch tags
	g.fetchTags(tempDir, dep)

	// Get commits between versions
	commits, err := g.getCommitLog(tempDir, dep)
	if err != nil {
		return nil, err
	}

	g.logger.Info("Found %d commits between versions for %s", len(commits), dep.Name)
	return commits, nil
}

// determineRepositoryURL derives the git repository URL from the module path
func (g *GitOperations) determineRepositoryURL(modulePath string) string {
	// Handle common repository hosts
	if strings.HasPrefix(modulePath, "github.com/") ||
		strings.HasPrefix(modulePath, "gitlab.com/") ||
		strings.HasPrefix(modulePath, "bitbucket.org/") {
		return "https://" + modulePath + ".git"
	}

	// Handle gopkg.in which uses a different URL format
	if strings.HasPrefix(modulePath, "gopkg.in/") {
		parts := strings.Split(modulePath, "/")
		if len(parts) >= 2 {
			// Convert gopkg.in/pkg.v3 to github.com/go-pkg/pkg
			// or gopkg.in/user/pkg.v3 to github.com/user/pkg
			if len(parts) == 2 { // gopkg.in/pkg.v3
				pkgParts := strings.Split(parts[1], ".")
				if len(pkgParts) >= 2 {
					return "https://github.com/go-" + pkgParts[0] + "/" + pkgParts[0] + ".git"
				}
			} else { // gopkg.in/user/pkg.v3
				pkgParts := strings.Split(parts[2], ".")
				if len(pkgParts) >= 2 {
					return "https://github.com/" + parts[1] + "/" + pkgParts[0] + ".git"
				}
			}
		}
	}

	// Best guess for unknown repository hosts
	g.logger.Warn("Unknown repository host for %s, using best guess", modulePath)
	return "https://" + modulePath + ".git"
}

// cloneRepository clones the git repository with minimal configuration
func (g *GitOperations) cloneRepository(repoURL, destDir string) error {
	cloneCmd := exec.Command("git", "clone",
		"--depth=1",
		"--no-tags",
		"--filter=blob:none", // Don't fetch file contents initially
		"--single-branch",    // Only clone a single branch
		repoURL, destDir)

	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repository %s: %w", repoURL, err)
	}

	return nil
}

// fetchTags attempts to fetch git tags for the repository
func (g *GitOperations) fetchTags(repoDir string, dep *models.Dependency) {
	// First, try to fetch all tags quietly
	fetchAllCmd := exec.Command("git", "fetch", "--tags", "--quiet", "origin")
	fetchAllCmd.Dir = repoDir

	if err := fetchAllCmd.Run(); err != nil {
		// If fetching all tags fails, try specific versions
		g.fetchSpecificTags(repoDir, dep)
	}
}

// fetchSpecificTags attempts to fetch specific version tags
func (g *GitOperations) fetchSpecificTags(repoDir string, dep *models.Dependency) {
	versionRefs := []string{
		"v" + dep.CurrentVersion,
		dep.CurrentVersion,
		"v" + dep.LatestVersion,
		dep.LatestVersion,
	}

	fetchSpecificCmd := exec.Command("git", "ls-remote", "--tags", "origin")
	fetchSpecificCmd.Dir = repoDir

	// Get available tags to check against
	tagsOutput, tagsErr := fetchSpecificCmd.Output()
	if tagsErr != nil {
		// If this fails too, try a more aggressive approach
		fetchCmd := exec.Command("git", "fetch", "--depth=100", "origin")
		fetchCmd.Dir = repoDir
		_ = fetchCmd.Run() // Last attempt, ignore errors and try to proceed
		return
	}

	// Look for tags that match our versions
	foundMatches := false
	tagsStr := string(tagsOutput)

	// Try to identify the right tag format from what's available in the repo
	for _, ref := range versionRefs {
		if strings.Contains(tagsStr, ref) {
			// Found a matching tag format, fetch just this one
			fetchCmd := exec.Command("git", "fetch", "--depth=1", "origin", "tag", ref)
			fetchCmd.Dir = repoDir
			_ = fetchCmd.Run() // Ignore errors here, we'll try to proceed anyway
			foundMatches = true
		}
	}

	if !foundMatches {
		// If no direct matches, try a more aggressive approach
		fetchCmd := exec.Command("git", "fetch", "--depth=100", "origin")
		fetchCmd.Dir = repoDir
		_ = fetchCmd.Run() // Last attempt, ignore errors and try to proceed
	}
}

// getCommitLog retrieves and parses the commit log between versions
func (g *GitOperations) getCommitLog(repoDir string, dep *models.Dependency) ([]models.CommitInfo, error) {
	versionFormats := []struct {
		current string
		latest  string
	}{
		{fmt.Sprintf("v%s", dep.CurrentVersion), fmt.Sprintf("v%s", dep.LatestVersion)},
		{dep.CurrentVersion, dep.LatestVersion},
		{fmt.Sprintf("refs/tags/v%s", dep.CurrentVersion), fmt.Sprintf("refs/tags/v%s", dep.LatestVersion)},
		{fmt.Sprintf("refs/tags/%s", dep.CurrentVersion), fmt.Sprintf("refs/tags/%s", dep.LatestVersion)},
		{fmt.Sprintf("%s^{}", dep.CurrentVersion), fmt.Sprintf("%s^{}", dep.LatestVersion)},
		{fmt.Sprintf("v%s^{}", dep.CurrentVersion), fmt.Sprintf("v%s^{}", dep.LatestVersion)},
	}

	// Try different version formats for git log
	for _, format := range versionFormats {
		output, err := g.runGitLog(repoDir, format.current, format.latest)
		if err == nil && len(output) > 0 {
			g.logger.Info("Found commits between %s and %s using format %s..%s",
				dep.CurrentVersion, dep.LatestVersion, format.current, format.latest)
			return g.parseCommitLog(output), nil
		}
	}

	// Try getting all recent commits as fallback
	output, err := g.getAllCommits(repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit history: %w", err)
	}

	if len(output) == 0 {
		g.logger.Info("No commits found between %s %s and %s", dep.Name, dep.CurrentVersion, dep.LatestVersion)
		return []models.CommitInfo{}, nil
	}

	return g.parseCommitLog(output), nil
}

// runGitLog executes git log with the specified version range
func (g *GitOperations) runGitLog(repoDir, fromVersion, toVersion string) ([]byte, error) {
	cmd := exec.Command("git", "log", "--pretty=format:%H|%s|%aI|%aN|%b",
		fmt.Sprintf("%s..%s", fromVersion, toVersion))
	cmd.Dir = repoDir
	return cmd.Output()
}

// getAllCommits gets a limited number of recent commits
func (g *GitOperations) getAllCommits(repoDir string) ([]byte, error) {
	cmd := exec.Command("git", "log", "--pretty=format:%H|%s|%aI|%aN|%b", "-n", "300")
	cmd.Dir = repoDir
	return cmd.Output()
}

// parseCommitLog parses the git log output into CommitInfo structs
func (g *GitOperations) parseCommitLog(output []byte) []models.CommitInfo {
	commits := []models.CommitInfo{}
	lines := strings.Split(string(output), "\n")
	maxCommits := 200
	processedCommits := 0

	for _, line := range lines {
		if line == "" {
			continue
		}

		if processedCommits >= maxCommits {
			g.logger.Info("Reached maximum commit limit (%d)", maxCommits)
			break
		}
		processedCommits++

		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 4 { // We need at least hash, message, date, and author
			continue
		}

		hash := parts[0]
		message := parts[1]
		dateStr := parts[2]

		// Extract full commit message if available
		fullMessage := message
		if len(parts) >= 5 && parts[4] != "" {
			fullMessage = message + "\n\n" + parts[4]
		}

		date, err := g.parseCommitDate(dateStr)
		if err != nil {
			date = time.Now() // Use current time as fallback
		}

		commits = append(commits, models.CommitInfo{
			Hash:    hash,
			Message: fullMessage,
			Date:    date,
		})
	}

	return commits
}

// parseCommitDate attempts to parse a commit date string with multiple formats
func (g *GitOperations) parseCommitDate(dateStr string) (time.Time, error) {
	// Try RFC3339 format first (git's default with %aI)
	if date, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return date, nil
	}

	// Try alternative formats
	alternativeFormats := []string{
		"2006-01-02 15:04:05 -0700",
		"Mon Jan 2 15:04:05 2006 -0700",
		time.RFC1123Z,
	}

	for _, format := range alternativeFormats {
		if date, err := time.Parse(format, dateStr); err == nil {
			return date, nil
		}
	}

	return time.Time{}, fmt.Errorf("could not parse date: %s", dateStr)
}
