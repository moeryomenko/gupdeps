package dependencies

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

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
	repo, err := g.cloneRepository(repoURL, tempDir)
	if err != nil {
		return nil, err
	}

	// Try to fetch tags
	if err := g.fetchTags(repo, dep); err != nil {
		g.logger.Warn("Failed to fetch tags for %s: %v", dep.Name, err)
		// Continue execution as we can still try to get commits
	}

	// Get commits between versions
	commits, err := g.getCommitLog(repo, dep)
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
func (g *GitOperations) cloneRepository(repoURL, destDir string) (*git.Repository, error) {
	// Clone with minimal configuration using go-git
	repo, err := git.PlainClone(destDir, false, &git.CloneOptions{
		URL:               repoURL,
		Depth:             1,
		SingleBranch:      true,
		NoCheckout:        false,
		RecurseSubmodules: git.NoRecurseSubmodules,
		Progress:          nil,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository %s: %w", repoURL, err)
	}

	return repo, nil
}

// fetchTags attempts to fetch git tags for the repository
func (g *GitOperations) fetchTags(repo *git.Repository, dep *models.Dependency) error {
	// Fetch all tags using go-git
	err := repo.Fetch(&git.FetchOptions{
		RefSpecs: []config.RefSpec{"refs/tags/*:refs/tags/*"},
		Force:    true,
	})

	// It's okay if we get "already up-to-date" error
	if err != nil && err != git.NoErrAlreadyUpToDate {
		g.logger.Warn("Failed to fetch tags for %s: %v", dep.Name, err)
		return err
	}

	return nil
}

// fetchSpecificTags attempts to fetch specific version tags
func (g *GitOperations) resolveVersionRef(repo *git.Repository, versionStr string) (*plumbing.Hash, error) {
	// Try different version reference formats
	versionRefs := []string{
		"refs/tags/" + versionStr,
		"refs/tags/v" + versionStr,
		versionStr,
		"v" + versionStr,
	}

	for _, ref := range versionRefs {
		// Try to resolve the reference
		hash, err := repo.ResolveRevision(plumbing.Revision(ref))
		if err == nil {
			return hash, nil
		}
	}

	return nil, fmt.Errorf("could not resolve version reference for %s", versionStr)
}

// getCommitLog retrieves and parses the commit log between versions
func (g *GitOperations) getCommitLog(repo *git.Repository, dep *models.Dependency) ([]models.CommitInfo, error) {
	// Try to fetch tags to ensure we have the version refs
	if err := g.fetchTags(repo, dep); err != nil {
		g.logger.Warn("Failed to fetch tags for %s: %v", dep.Name, err)
		// Continue execution as we can still try to resolve version references
	}

	// Get the hash for the current and latest versions
	currentHash, currentErr := g.resolveVersionRef(repo, dep.CurrentVersion)
	latestHash, latestErr := g.resolveVersionRef(repo, dep.LatestVersion)

	// If we can't resolve the refs, try getting recent commits instead
	if currentErr != nil || latestErr != nil {
		g.logger.Warn("Could not resolve version references: current=%v, latest=%v", currentErr, latestErr)
		return g.getRecentCommits(repo, 300)
	}

	// Get the commit logs between the two versions
	commits, err := g.getCommitsBetweenHashes(repo, *currentHash, *latestHash)
	if err != nil {
		g.logger.Warn("Failed to get commits between versions: %v", err)
		return g.getRecentCommits(repo, 300)
	}

	if len(commits) == 0 {
		g.logger.Info("No commits found between %s %s and %s", dep.Name, dep.CurrentVersion, dep.LatestVersion)
	}

	return commits, nil
}

// runGitLog executes git log with the specified version range
func (g *GitOperations) getCommitsBetweenHashes(
	repo *git.Repository,
	from, to plumbing.Hash,
) ([]models.CommitInfo, error) {
	// Get commit objects for from and to
	fromCommit, err := repo.CommitObject(from)
	if err != nil {
		return nil, fmt.Errorf("failed to get 'from' commit: %w", err)
	}

	toCommit, err := repo.CommitObject(to)
	if err != nil {
		return nil, fmt.Errorf("failed to get 'to' commit: %w", err)
	}

	// Use commit log to get commits between versions
	logOpts := &git.LogOptions{
		From: toCommit.Hash,
	}

	commitIter, err := repo.Log(logOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}

	commits := []models.CommitInfo{}
	maxCommits := 200
	processedCommits := 0

	err = commitIter.ForEach(func(c *object.Commit) error {
		// Stop when we reach the "from" commit
		if c.Hash == fromCommit.Hash {
			return fmt.Errorf("reached boundary commit")
		}

		if processedCommits >= maxCommits {
			return fmt.Errorf("reached maximum commit limit (%d)", maxCommits)
		}

		processedCommits++

		commits = append(commits, models.CommitInfo{
			Hash:    c.Hash.String(),
			Message: c.Message,
			Date:    c.Author.When,
		})

		return nil
	})

	// This error is expected when we reach a boundary condition
	if err != nil && err.Error() != "reached boundary commit" &&
		!strings.Contains(err.Error(), "reached maximum commit limit") {
		return nil, fmt.Errorf("error iterating commits: %w", err)
	}

	return commits, nil
}

// getAllCommits gets a limited number of recent commits
func (g *GitOperations) getRecentCommits(repo *git.Repository, limit int) ([]models.CommitInfo, error) {
	// Get HEAD reference
	headRef, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	// Get commit history from HEAD
	commitIter, err := repo.Log(&git.LogOptions{
		From: headRef.Hash(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}

	commits := []models.CommitInfo{}
	processedCommits := 0

	err = commitIter.ForEach(func(c *object.Commit) error {
		if processedCommits >= limit {
			return fmt.Errorf("reached maximum commit limit (%d)", limit)
		}

		processedCommits++

		commits = append(commits, models.CommitInfo{
			Hash:    c.Hash.String(),
			Message: c.Message,
			Date:    c.Author.When,
		})

		return nil
	})

	// This error is expected when we reach the limit
	if err != nil && !strings.Contains(err.Error(), "reached maximum commit limit") {
		return nil, fmt.Errorf("error iterating commits: %w", err)
	}

	return commits, nil
}
