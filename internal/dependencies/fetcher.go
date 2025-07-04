package dependencies

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/moeryomenko/gupdeps/internal/models"
	"github.com/moeryomenko/gupdeps/internal/utils"
)

// DependencyFetcher handles retrieving dependency information
type DependencyFetcher struct {
	projectPath string
	httpClient  *http.Client
	logger      *utils.Logger
}

// NewDependencyFetcher creates a new dependency fetcher
func NewDependencyFetcher(projectPath string, logger *utils.Logger) *DependencyFetcher {
	return &DependencyFetcher{
		projectPath: projectPath,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// GetDependencies retrieves direct dependencies from go.mod
func (df *DependencyFetcher) GetDependencies() ([]*models.Dependency, error) {
	// First, get direct dependencies only
	directDeps, err := df.getDirectDependencies()
	if err != nil {
		return nil, fmt.Errorf("failed to get direct dependencies: %w", err)
	}

	// Get version info for direct dependencies
	cmd := exec.Command("go", "list", "-m", "-json", "all")
	cmd.Dir = df.projectPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get dependency versions: %w", err)
	}

	moduleVersions := df.parseModuleVersions(output)

	// Build dependency list for direct dependencies only
	var dependencies []*models.Dependency
	for depName := range directDeps {
		if version, exists := moduleVersions[depName]; exists {
			dependencies = append(dependencies, &models.Dependency{
				Name:           depName,
				CurrentVersion: version,
			})
		}
	}

	// Sort for consistent output
	sort.Slice(dependencies, func(i, j int) bool {
		return dependencies[i].Name < dependencies[j].Name
	})

	return dependencies, nil
}

// parseModuleVersions parses the output from go list and returns a map of module paths to versions
func (df *DependencyFetcher) parseModuleVersions(output []byte) map[string]string {
	moduleVersions := make(map[string]string)
	decoder := json.NewDecoder(strings.NewReader(string(output)))

	for {
		var module struct {
			Path    string `json:"Path"`
			Version string `json:"Version"`
			Main    bool   `json:"Main"`
		}

		if err := decoder.Decode(&module); err == io.EOF {
			break
		} else if err != nil {
			df.logger.Warn("Failed to decode module info: %v", err)
			continue
		}

		if !module.Main && module.Version != "" {
			moduleVersions[module.Path] = module.Version
		}
	}

	return moduleVersions
}

// getDirectDependencies parses go.mod file to get only direct dependencies
// openGoModFile opens the go.mod file for reading
func (df *DependencyFetcher) openGoModFile() (*os.File, error) {
	goModPath := fmt.Sprintf("%s/go.mod", df.projectPath)
	file, err := os.Open(goModPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open go.mod: %w", err)
	}
	return file, nil
}

// processGoModLine processes a single line from go.mod file
func (df *DependencyFetcher) processGoModLine(line string, directDeps map[string]bool, inRequireBlock *bool) bool {
	// Skip empty lines and comments
	if line == "" || strings.HasPrefix(line, "//") {
		return *inRequireBlock
	}

	// Check for require block start
	if strings.HasPrefix(line, "require (") {
		*inRequireBlock = true
		return *inRequireBlock
	}

	// Check for require block end
	if *inRequireBlock && line == ")" {
		*inRequireBlock = false
		return *inRequireBlock
	}

	// Handle single-line require
	if strings.HasPrefix(line, "require ") && !strings.HasSuffix(line, "(") {
		df.parseDependencyLine(line, directDeps)
		return *inRequireBlock
	}

	// Handle require block content
	if *inRequireBlock {
		df.parseDependencyLine(line, directDeps)
	}

	return *inRequireBlock
}

func (df *DependencyFetcher) getDirectDependencies() (map[string]bool, error) {
	file, err := df.openGoModFile()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	directDeps := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	inRequireBlock := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		df.processGoModLine(line, directDeps, &inRequireBlock)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading go.mod: %w", err)
	}

	return directDeps, nil
}

// parseDependencyLine extracts dependency information from a line in go.mod
func (df *DependencyFetcher) parseDependencyLine(line string, directDeps map[string]bool) {
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		depName := parts[0]
		// For single-line require statements, the module name is in position 1
		if strings.HasPrefix(line, "require ") {
			depName = parts[1]
		}
		// Skip indirect dependencies
		if !strings.Contains(line, "// indirect") {
			directDeps[depName] = true
		}
	}
}

// GetLatestVersion fetches the latest version for a dependency
func (df *DependencyFetcher) GetLatestVersion(dep *models.Dependency) error {
	cmd := exec.Command("go", "list", "-m", "-versions", dep.Name)
	cmd.Dir = df.projectPath

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get versions for %s: %w", dep.Name, err)
	}

	versionLine := strings.TrimSpace(string(output))
	parts := strings.Fields(versionLine)

	if len(parts) < 2 {
		return fmt.Errorf("no versions found for %s", dep.Name)
	}

	// Get the latest version (last in the list)
	versions := parts[1:]
	if len(versions) > 0 {
		dep.LatestVersion = versions[len(versions)-1]
		dep.UpdateNeeded = dep.CurrentVersion != dep.LatestVersion
	}

	return nil
}
