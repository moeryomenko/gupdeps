package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/moeryomenko/gupdeps/internal/dependencies"
	"github.com/moeryomenko/gupdeps/internal/models"
	"github.com/moeryomenko/gupdeps/internal/utils"
)

func main() {
	// Parse command-line flags
	projectPath := flag.String("path", ".", "Path to the Go project")
	interactive := flag.Bool("interactive", false, "Run in interactive mode")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	help := flag.Bool("help", false, "Show help information")

	flag.Parse()

	if *help {
		printHelp()
		os.Exit(0)
	}

	// Initialize logger
	logger := utils.NewLogger(*verbose)

	// Create dependency updater
	updater := dependencies.NewDependencyUpdater(*projectPath, logger)

	if *interactive {
		logger.Print("🎮 Running in interactive mode...")
		if err := runInteractiveMode(updater, logger); err != nil {
			logger.Error("Interactive mode failed: %v", err)
			os.Exit(1)
		}
	} else {
		logger.Print("🤖 Running in automatic mode...")
		if err := runAutomaticMode(updater, logger); err != nil {
			logger.Error("Update failed: %v", err)
			os.Exit(1)
		}
	}
}

func printHelp() {
	fmt.Println("Dependency Updater - Analyze and update Go dependencies")
	fmt.Println("\nUsage:")
	fmt.Println("  update-deps [flags]")
	fmt.Println("\nFlags:")
	fmt.Println("  -path string        Path to the Go project (default \".\")")
	fmt.Println("  -interactive        Run in interactive mode")
	fmt.Println("  -verbose            Enable verbose logging")
	fmt.Println("  -help               Show this help information")
	fmt.Println("\nExamples:")
	fmt.Println("  update-deps -path ./my-project")
	fmt.Println("  update-deps -interactive -verbose")
}

// fetchAndDisplayDependencies gets dependencies and displays them
func fetchAndDisplayDependencies(
	updater *dependencies.DependencyUpdater,
	logger *utils.Logger,
) ([]*models.Dependency, error) {
	logger.Print("🔍 Fetching direct dependencies from go.mod...")
	deps, err := updater.GetAllDependencies()
	if err != nil {
		return nil, fmt.Errorf("failed to get dependencies: %w", err)
	}

	logger.Print("Found %d direct dependencies\n", len(deps))

	// Display found dependencies
	if len(deps) > 0 {
		logger.Print("📋 Direct dependencies:")
		for _, dep := range deps {
			logger.Print("  %s@%s", dep.Name, dep.CurrentVersion)
		}
		logger.Print("")
	}

	return deps, nil
}

// analyzeDependencies analyzes each dependency and returns approved/rejected updates
func analyzeDependencies(updater *dependencies.DependencyUpdater, logger *utils.Logger, deps []*models.Dependency) (
	approvedList, rejectedList []*models.UpdateAnalysis, err error,
) {
	logger.Print("📡 Checking for updates...")
	var approvedUpdates []*models.UpdateAnalysis
	var rejectedUpdates []*models.UpdateAnalysis

	for _, dep := range deps {
		logger.Print("🔍 Analyzing %s (%s)...", dep.Name, dep.CurrentVersion)

		analysis, err := updater.AnalyzeDependency(dep)
		if err != nil {
			logger.Warn("Could not analyze %s: %v", dep.Name, err)
			continue
		}

		if !dep.UpdateNeeded {
			continue
		}

		logger.Print("  Current: %s → Latest: %s", dep.CurrentVersion, dep.LatestVersion)

		if analysis.ShouldUpdate {
			approvedUpdates = append(approvedUpdates, analysis)
			logger.Print("  ✅ Approved: %s", analysis.UpdateReason)
		} else {
			rejectedUpdates = append(rejectedUpdates, analysis)
			logger.Print("  ❌ Rejected: %s", analysis.RejectionReason)
		}
	}

	return approvedUpdates, rejectedUpdates, nil
}

// applyUpdates applies the approved updates
func applyUpdates(
	updater *dependencies.DependencyUpdater,
	logger *utils.Logger,
	approvedUpdates []*models.UpdateAnalysis,
) error {
	if len(approvedUpdates) == 0 {
		return nil
	}

	logger.Print("🚀 Applying approved updates...")
	for _, analysis := range approvedUpdates {
		if err := updater.ApplyUpdate(analysis.Dependency); err != nil {
			logger.Error("Failed to update %s: %v", analysis.Dependency.Name, err)
		}
	}

	// Run go mod tidy to clean up
	logger.Print("\n🧹 Running go mod tidy...")
	if err := updater.RunModTidy(); err != nil {
		logger.Warn("go mod tidy failed: %v", err)
	}

	return nil
}

// displayRejectedUpdates prints info about rejected updates
func displayRejectedUpdates(logger *utils.Logger, rejectedUpdates []*models.UpdateAnalysis) {
	if len(rejectedUpdates) == 0 {
		return
	}

	logger.Print("\n📝 Rejected Updates (manual review recommended):")
	for _, analysis := range rejectedUpdates {
		logger.Print("  %s (%s → %s): %s",
			analysis.Dependency.Name,
			analysis.Dependency.CurrentVersion,
			analysis.Dependency.LatestVersion,
			analysis.RejectionReason)
	}
}

func runAutomaticMode(updater *dependencies.DependencyUpdater, logger *utils.Logger) error {
	deps, err := fetchAndDisplayDependencies(updater, logger)
	if err != nil {
		return err
	}

	approvedUpdates, rejectedUpdates, err := analyzeDependencies(updater, logger, deps)
	if err != nil {
		return err
	}

	// Display summary
	logger.Print("\n📋 Update Summary:")
	logger.Print("  Approved: %d updates", len(approvedUpdates))
	logger.Print("  Rejected: %d updates", len(rejectedUpdates))
	logger.Print("")

	if err := applyUpdates(updater, logger, approvedUpdates); err != nil {
		return err
	}

	displayRejectedUpdates(logger, rejectedUpdates)

	return nil
}

// displayDependencyInfo shows detailed information about a dependency update
func displayDependencyInfo(logger *utils.Logger, dep *models.Dependency, analysis *models.UpdateAnalysis) {
	logger.Print("\n📦 %s", dep.Name)
	logger.Print("Current: %s → Latest: %s", dep.CurrentVersion, dep.LatestVersion)

	if analysis.ShouldUpdate {
		logger.Print("Analysis: ✅ %s", analysis.UpdateReason)
	} else {
		logger.Print("Analysis: ❌ %s", analysis.RejectionReason)
	}

	logger.Print("Recent commits:")
	displayLimit := 5
	for i, commit := range analysis.Commits {
		if i >= displayLimit { // Show only first few commits
			break
		}
		logger.Print("  - %s", commit.Message)
	}
}

// processDependencyInteractive handles user interaction for a single dependency update
func processDependencyInteractive(
	updater *dependencies.DependencyUpdater,
	logger *utils.Logger,
	dep *models.Dependency,
	analysis *models.UpdateAnalysis,
	reader *bufio.Reader,
) (bool, error) {
	logger.Print("\nApply this update? (y/n/q): ")
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	switch response {
	case "y", "yes":
		if err := updater.ApplyUpdate(dep); err != nil {
			logger.Error("Failed: %v", err)
		}
	case "q", "quit":
		return true, nil // Signal to quit
	default:
		logger.Print("⏭️  Skipped")
	}

	return false, nil
}

func runInteractiveMode(updater *dependencies.DependencyUpdater, logger *utils.Logger) error {
	deps, err := updater.GetAllDependencies()
	if err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)

	for _, dep := range deps {
		analysis, err := updater.AnalyzeDependency(dep)
		if err != nil {
			logger.Warn("Could not analyze %s: %v", dep.Name, err)
			continue
		}

		if !dep.UpdateNeeded {
			continue
		}

		displayDependencyInfo(logger, dep, analysis)

		quit, err := processDependencyInteractive(updater, logger, dep, analysis, reader)
		if err != nil {
			return err
		}
		if quit {
			return nil
		}
	}

	// Run go mod tidy at the end of the session
	logger.Print("\n🧹 Running go mod tidy...")
	if err := updater.RunModTidy(); err != nil {
		logger.Warn("go mod tidy failed: %v", err)
	}

	return nil
}
