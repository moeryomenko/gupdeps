package dependencies

import (
	"fmt"
	"os/exec"

	"github.com/moeryomenko/gupdeps/internal/models"
	"github.com/moeryomenko/gupdeps/internal/utils"
)

// DependencyUpdater coordinates the dependency update process
type DependencyUpdater struct {
	projectPath string
	fetcher     *DependencyFetcher
	gitOps      *GitOperations
	analyzer    *CommitAnalyzer
	logger      *utils.Logger
}

// NewDependencyUpdater creates a new dependency updater
func NewDependencyUpdater(projectPath string, logger *utils.Logger) *DependencyUpdater {
	return &DependencyUpdater{
		projectPath: projectPath,
		fetcher:     NewDependencyFetcher(projectPath, logger),
		gitOps:      NewGitOperations(logger),
		analyzer:    NewCommitAnalyzer(logger),
		logger:      logger,
	}
}

// ApplyUpdate applies the update for a dependency
func (du *DependencyUpdater) ApplyUpdate(dep *models.Dependency) error {
	cmd := exec.Command("go", "get", dep.Name+"@"+dep.LatestVersion)
	cmd.Dir = du.projectPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update %s: %w\nOutput: %s", dep.Name, err, string(output))
	}

	du.logger.Success("Updated %s from %s to %s", dep.Name, dep.CurrentVersion, dep.LatestVersion)
	return nil
}

// RunModTidy runs go mod tidy to clean up dependencies
func (du *DependencyUpdater) RunModTidy() error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = du.projectPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go mod tidy failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// AnalyzeDependency performs analysis on a single dependency
func (du *DependencyUpdater) AnalyzeDependency(dep *models.Dependency) (*models.UpdateAnalysis, error) {
	// Check if update is needed
	if err := du.fetcher.GetLatestVersion(dep); err != nil {
		return nil, fmt.Errorf("failed to get latest version: %w", err)
	}

	if !dep.UpdateNeeded {
		du.logger.Info("No update needed for %s (already at %s)", dep.Name, dep.CurrentVersion)
		return &models.UpdateAnalysis{
			Dependency:   dep,
			ShouldUpdate: false,
		}, nil
	}

	// Get commits between versions
	commits, err := du.gitOps.GetCommitsBetweenVersions(dep)
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %w", err)
	}

	// Analyze the changes
	analysis := du.analyzer.AnalyzeUpdate(dep, commits)
	return analysis, nil
}

// GetAllDependencies returns all direct dependencies
func (du *DependencyUpdater) GetAllDependencies() ([]*models.Dependency, error) {
	return du.fetcher.GetDependencies()
}
