package runner

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/purell"
	"github.com/root4loot/goutils/httputil"
	"github.com/root4loot/relog"
)

var Log = relog.NewLogger("npmjack")

type Runner struct {
	Options Options         // options for the runner
	client  *http.Client    // http client
	Results chan Result     // channel to receive results
	Visited map[string]bool // map of visited urls
}

type Result struct {
	RequestURL string    // url that was requested
	StatusCode int       // status code of the response
	Packages   []Package // packages found in the response
	Error      error     // error if anys
}

type Package struct {
	Name      string // package name
	Namespace string // package namespace
	Claimed   bool   // whether the package is claimed or not
}

type Results struct {
	Results []Result
}

// JSON structures for parsing package files
type PackageJSON struct {
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	BundledDependencies  []string          `json:"bundledDependencies"`
}

type PackageLockJSON struct {
	Packages map[string]PackageLockEntry `json:"packages"`
}

type PackageLockEntry struct {
	Dependencies map[string]string `json:"dependencies"`
}

var (
	// JavaScript patterns
	jsImportRegex         = regexp.MustCompile(`\b(?:require|import)\s*\(?['"]([^'"]+)['"]\)?`)
	jsImportFromRegex     = regexp.MustCompile(`\bfrom\s+['"]([^'"]+)['"]`)
	jsRequireResolveRegex = regexp.MustCompile(`\brequire\.resolve\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	jsDynamicImportRegex  = regexp.MustCompile(`\bimport\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	amdDefineRegex        = regexp.MustCompile(`\bdefine\s*\(\s*\[([^\]]+)\]`)
	amdRequireRegex       = regexp.MustCompile(`\brequire\s*\(\s*\[([^\]]+)\]`)

	// Documentation patterns
	npmInstallRegex = regexp.MustCompile(`npm\s+install\s+([a-zA-Z0-9@/_-]+)`)
	yarnAddRegex    = regexp.MustCompile(`yarn\s+add\s+([a-zA-Z0-9@/_-]+)`)

	// Webpack/config patterns for string literals in arrays/objects
	stringLiteralRegex = regexp.MustCompile(`['"](@?[a-zA-Z0-9/_-]+(?:@[a-zA-Z0-9/_-]+)?/[a-zA-Z0-9/_-]+|[a-zA-Z0-9-]+(?:-[a-zA-Z0-9]+)*)['"]:?`)
	loaderRegex        = regexp.MustCompile(`loader:\s*['"]([^'"]+)['"]`)
	useArrayRegex      = regexp.MustCompile(`use:\s*\[([^\]]+)\]`)
	presetPluginRegex  = regexp.MustCompile(`(?:presets?|plugins?):\s*\[([^\]]+)\]`)

	// File extensions to scan
	jsExtensions = []string{
		".js", ".mjs", ".cjs", ".jsx", ".ts", ".tsx", ".vue", ".html", ".htm", ".svelte"}
	jsonExtensions = []string{
		".json"}
	configExtensions = []string{
		".config.js", ".config.ts", ".config.json"}
	cicdExtensions = []string{
		".yml", ".yaml", ".sh", ".bash"}
	docExtensions = []string{
		".md", ".rst", ".txt"}
)

// Options contains options for the runner
type Options struct {
	Concurrency int      // number of concurrent requests
	Timeout     int      // timeout in seconds
	Delay       int      // delay in seconds
	DelayJitter int      // delay jitter in seconds
	Verbose     bool     // verbose logging
	Silence     bool     // suppress output from console
	UserAgent   string   // user agent
	Resolvers   []string // DNS resolvers
}

// DefaultOptions returns default options
func DefaultOptions() *Options {
	return &Options{
		Concurrency: 10,
		Timeout:     30,
		Delay:       0,
		DelayJitter: 0,
		UserAgent:   "npmjack",
	}
}

// NewRunner returns a new runner
func NewRunner() *Runner {
	options := DefaultOptions()
	SetLogLevel(options)
	var client *http.Client

	client, _ = httputil.ClientWithOptionalResolvers()
	client.Transport = &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		MaxIdleConnsPerHost:   options.Concurrency,
		ResponseHeaderTimeout: time.Duration(options.Timeout) * time.Second,
	}
	client.Timeout = time.Duration(options.Timeout) * time.Second

	return &Runner{
		Results: make(chan Result),
		Visited: make(map[string]bool),
		Options: *options,
		client:  client,
	}
}

// Run starts the runner
func (r *Runner) Run(urls ...string) {
	// defer close(r.Results)

	if r.Options.Resolvers != nil {
		r.client, _ = httputil.ClientWithOptionalResolvers(r.Options.Resolvers...)
	}

	sem := make(chan struct{}, r.Options.Concurrency)
	var wg sync.WaitGroup

	for _, url := range urls {
		Log.Debugf("Running on %s", url)

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(r.Options.Timeout)*time.Second)
		defer cancel()
		url, err := normalizeURLString(url)
		url = trimURLParams(url)
		if err != nil {
			Log.Warningf("%v", err.Error())
			continue
		}

		if !r.Visited[url] {
			r.Visited[url] = true
			if r.hasFileExtension(url) && !r.hasValidExtension(url) {
				continue
			}

			sem <- struct{}{}
			wg.Add(1)
			go func(u string) {
				defer func() { <-sem }()
				defer wg.Done()
				r.Results <- r.scrapePackages(ctx, u, r.client)
				time.Sleep(time.Millisecond * 10) // make room for processing results
			}(url)

			time.Sleep(r.getDelay() * time.Millisecond) // delay between requests
		}
	}
	wg.Wait()
}

// scrapePackages scrapes public NPM packages from a URL
func (r *Runner) scrapePackages(ctx context.Context, url string, client *http.Client) Result {
	Log.Debugf("Scraping packages from %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		Log.Warningf("%v", err.Error())
		return Result{RequestURL: url, Error: err}
	}

	if r.Options.UserAgent != "" {
		req.Header.Add("User-Agent", r.Options.UserAgent)
	}

	resp, err := client.Do(req)
	if err != nil {
		Log.Warningf("%v", err.Error())
		return Result{RequestURL: url, Error: err}
	}

	defer resp.Body.Close()
	res := Result{RequestURL: url, StatusCode: resp.StatusCode, Error: err}

	seenPackages := make(map[string]bool)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log.Warningf("Error reading response body: %v", err)
		return Result{RequestURL: url, Error: err}
	}

	// Detect file type and extract packages accordingly
	packages := r.extractPackages(url, string(body))

	for _, pkg := range packages {
		if !seenPackages[pkg.Name] {
			if r.isPackageClaimed(pkg.Name) {
				pkg.Claimed = true
			}
			res.Packages = append(res.Packages, pkg)
			seenPackages[pkg.Name] = true
		}
	}

	return res
}

// extractPackages extracts NPM packages from content based on file type
func (r *Runner) extractPackages(url, content string) []Package {
	var packages []Package

	// Determine file type from URL and apply appropriate extractors
	if r.isJSONFile(url) {
		packages = append(packages, r.extractFromJSON(content)...)
	}

	if r.isConfigFile(url) {
		packages = append(packages, r.extractFromConfigFile(content)...)
	}

	if r.isCICDFile(url) {
		packages = append(packages, r.extractFromCICD(content)...)
	}

	if r.isDocFile(url) {
		packages = append(packages, r.extractFromDocumentation(content)...)
	}

	// always try JavaScript patterns
	packages = append(packages, r.extractFromJavaScript(content)...)

	return packages
}

// isJSONFile checks if the URL represents a JSON file
func (r *Runner) isJSONFile(url string) bool {
	for _, ext := range jsonExtensions {
		if strings.HasSuffix(url, ext) {
			return true
		}
	}
	// Also check for common JSON filenames
	lowerURL := strings.ToLower(url)
	return strings.Contains(lowerURL, "package.json") ||
		strings.Contains(lowerURL, "package-lock.json") ||
		strings.Contains(lowerURL, "yarn.lock")
}

// isConfigFile checks if the URL represents a configuration file
func (r *Runner) isConfigFile(url string) bool {
	lowerURL := strings.ToLower(url)
	configFiles := []string{
		"webpack.config", "rollup.config", "vite.config", "babel.config",
		"jest.config", "prettier.config", "eslint", ".babelrc", ".prettierrc",
		"tsconfig.json", "jsconfig.json", ".eslintrc", ".stylelintrc",
	}

	for _, config := range configFiles {
		if strings.Contains(lowerURL, config) {
			return true
		}
	}
	return false
}

// isCICDFile checks if the URL represents a CI/CD file
func (r *Runner) isCICDFile(url string) bool {
	lowerURL := strings.ToLower(url)
	cicdFiles := []string{
		".github/workflows", ".gitlab-ci", "dockerfile", "docker-compose",
		".travis", ".circleci", "makefile",
	}

	for _, cicd := range cicdFiles {
		if strings.Contains(lowerURL, cicd) {
			return true
		}
	}

	for _, ext := range cicdExtensions {
		if strings.HasSuffix(lowerURL, ext) {
			return true
		}
	}

	if strings.HasSuffix(lowerURL, "makefile") || strings.HasSuffix(lowerURL, "makefile.") {
		return true
	}

	return false
}

// isDocFile checks if the URL represents a documentation file
func (r *Runner) isDocFile(url string) bool {
	for _, ext := range docExtensions {
		if strings.HasSuffix(strings.ToLower(url), ext) {
			return true
		}
	}
	return false
}

// extractFromJSON extracts packages from JSON files
func (r *Runner) extractFromJSON(content string) []Package {
	var packages []Package

	// Try package.json format
	if pkgJSON := r.parsePackageJSON(content); pkgJSON != nil {
		packages = append(packages, r.extractFromPackageJSON(pkgJSON)...)
	}

	// Try package-lock.json format
	if lockJSON := r.parsePackageLockJSON(content); lockJSON != nil {
		packages = append(packages, r.extractFromPackageLockJSON(lockJSON)...)
	}

	// Try yarn.lock format (regex-based since it's not JSON)
	packages = append(packages, r.extractFromYarnLock(content)...)

	return packages
}

// parsePackageJSON attempts to parse content as package.json
func (r *Runner) parsePackageJSON(content string) *PackageJSON {
	var pkg PackageJSON
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return nil
	}
	return &pkg
}

// parsePackageLockJSON attempts to parse content as package-lock.json
func (r *Runner) parsePackageLockJSON(content string) *PackageLockJSON {
	var lockFile PackageLockJSON
	if err := json.Unmarshal([]byte(content), &lockFile); err != nil {
		return nil
	}
	return &lockFile
}

// extractFromPackageJSON extracts packages from parsed package.json
func (r *Runner) extractFromPackageJSON(pkg *PackageJSON) []Package {
	var packages []Package

	// Extract from all dependency types
	dependencies := []map[string]string{
		pkg.Dependencies,
		pkg.DevDependencies,
		pkg.PeerDependencies,
		pkg.OptionalDependencies,
	}

	for _, deps := range dependencies {
		for name := range deps {
			packages = append(packages, r.createPackageFromName(name))
		}
	}

	// Extract bundled dependencies
	for _, name := range pkg.BundledDependencies {
		packages = append(packages, r.createPackageFromName(name))
	}

	return packages
}

// extractFromPackageLockJSON extracts packages from parsed package-lock.json
func (r *Runner) extractFromPackageLockJSON(lockFile *PackageLockJSON) []Package {
	var packages []Package

	for packagePath, entry := range lockFile.Packages {
		// Extract package name from node_modules path
		if strings.Contains(packagePath, "node_modules/") {
			parts := strings.Split(packagePath, "node_modules/")
			if len(parts) > 1 {
				name := parts[len(parts)-1]
				packages = append(packages, r.createPackageFromName(name))
			}
		}

		// Extract dependencies
		for depName := range entry.Dependencies {
			packages = append(packages, r.createPackageFromName(depName))
		}
	}

	return packages
}

// extractFromYarnLock extracts packages from yarn.lock content
func (r *Runner) extractFromYarnLock(content string) []Package {
	var packages []Package

	// Yarn lock format patterns
	scopedPackageRegex := regexp.MustCompile(`^"(@[^/]+/[^@"]+)(?:@[^"]*)?":`)
	normalPackageRegex := regexp.MustCompile(`^"([^@"]+)(?:@[^"]*)?":`)
	// Dependencies with or without quotes (in dependencies block)
	dependencyRegex := regexp.MustCompile(`^\s{4}"?(@?[a-zA-Z0-9/@_-]+)"?\s+"([^"]+)"`)

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		originalLine := line
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Scoped package declarations: "@babel/core@version":
		if matches := scopedPackageRegex.FindStringSubmatch(line); matches != nil {
			name := matches[1]
			packages = append(packages, r.createPackageFromName(name))
			continue
		}

		// Normal package declarations: "package-name@version":
		if matches := normalPackageRegex.FindStringSubmatch(line); matches != nil {
			name := matches[1]
			packages = append(packages, r.createPackageFromName(name))
			continue
		}

		// Dependency entries (4 spaces indent, no quotes on package name)
		if strings.HasPrefix(originalLine, "    ") && !strings.HasPrefix(line, "version") &&
			!strings.HasPrefix(line, "resolved") && !strings.HasPrefix(line, "dependencies") &&
			!strings.HasPrefix(line, "integrity") {
			if matches := dependencyRegex.FindStringSubmatch(originalLine); matches != nil {
				name := matches[1]
				if !r.isBuiltinModule(name) && r.looksLikePackageName(name) {
					packages = append(packages, r.createPackageFromName(name))
				}
			}
		}
	}

	return packages
}

// extractFromJavaScript extracts packages from JavaScript content using multiple patterns
func (r *Runner) extractFromJavaScript(content string) []Package {
	var packages []Package

	// Standard require/import patterns
	patterns := []*regexp.Regexp{
		jsImportRegex,
		jsImportFromRegex,
		jsRequireResolveRegex,
		jsDynamicImportRegex,
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 && !r.isBuiltinModule(match[1]) {
				packages = append(packages, r.createPackageFromName(match[1]))
			}
		}
	}

	// AMD define patterns
	amdDefineMatches := amdDefineRegex.FindAllStringSubmatch(content, -1)
	for _, match := range amdDefineMatches {
		if len(match) > 1 {
			deps := r.parseAMDDependencies(match[1])
			for _, dep := range deps {
				packages = append(packages, r.createPackageFromName(dep))
			}
		}
	}

	// AMD require patterns
	amdRequireMatches := amdRequireRegex.FindAllStringSubmatch(content, -1)
	for _, match := range amdRequireMatches {
		if len(match) > 1 {
			deps := r.parseAMDDependencies(match[1])
			for _, dep := range deps {
				packages = append(packages, r.createPackageFromName(dep))
			}
		}
	}

	// RequireJS config paths
	requireConfigRegex := regexp.MustCompile(`paths:\s*\{([^}]+)\}`)
	configMatches := requireConfigRegex.FindAllStringSubmatch(content, -1)
	for _, match := range configMatches {
		if len(match) > 1 {
			// Extract package names from paths config
			pathRegex := regexp.MustCompile(`['"]([^'"]+)['"]\s*:\s*['"][^'"]+['"]`)
			pathMatches := pathRegex.FindAllStringSubmatch(match[1], -1)
			for _, pm := range pathMatches {
				if len(pm) > 1 && !r.isBuiltinModule(pm[1]) {
					packages = append(packages, r.createPackageFromName(pm[1]))
				}
			}
		}
	}

	// Documentation patterns (npm install, yarn add)
	docPatterns := []*regexp.Regexp{
		npmInstallRegex,
		yarnAddRegex,
	}

	for _, pattern := range docPatterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 && !r.isBuiltinModule(match[1]) {
				packages = append(packages, r.createPackageFromName(match[1]))
			}
		}
	}

	return packages
}

// parseAMDDependencies parses AMD dependency arrays like ['jquery', 'underscore']
func (r *Runner) parseAMDDependencies(depString string) []string {
	var dependencies []string

	depString = strings.ReplaceAll(depString, " ", "")
	parts := strings.Split(depString, ",")

	for _, part := range parts {
		part = strings.Trim(part, `'"`)
		if part != "" && !r.isBuiltinModule(part) {
			dependencies = append(dependencies, part)
		}
	}

	return dependencies
}

// extractFromConfigFile extracts packages from configuration files
func (r *Runner) extractFromConfigFile(content string) []Package {
	var packages []Package

	// Try JSON parsing first (for .json)
	if strings.Contains(content, "{") && strings.Contains(content, "}") {
		packages = append(packages, r.extractFromJSON(content)...)
	}

	packages = append(packages, r.extractFromJavaScript(content)...)

	// Extract loaders and plugins from webpack-style configs
	configPatterns := []*regexp.Regexp{
		loaderRegex,
		stringLiteralRegex,
	}

	for _, pattern := range configPatterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				name := match[1]
				// Clean up the name and check if it's a package
				name = strings.Trim(name, `'"`)
				if r.looksLikePackageName(name) && !r.isBuiltinModule(name) {
					packages = append(packages, r.createPackageFromName(name))
				}
			}
		}
	}

	// Extract from use arrays and preset/plugin arrays
	arrayPatterns := []*regexp.Regexp{
		useArrayRegex,
		presetPluginRegex,
	}

	for _, pattern := range arrayPatterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				// Parse array contents
				arrayContent := match[1]
				// Extract quoted strings from array
				stringMatches := regexp.MustCompile(`['"]([^'"]+)['"]`).FindAllStringSubmatch(arrayContent, -1)
				for _, sm := range stringMatches {
					if len(sm) > 1 {
						name := sm[1]
						if r.looksLikePackageName(name) && !r.isBuiltinModule(name) {
							packages = append(packages, r.createPackageFromName(name))
						}
					}
				}
			}
		}
	}

	return packages
}

// extractFromCICD extracts packages from CI/CD files
func (r *Runner) extractFromCICD(content string) []Package {
	var packages []Package

	lines := strings.Split(content, "\n")

	for _, line := range lines {
		// Skip comments and empty lines
		if strings.HasPrefix(strings.TrimSpace(line), "#") || strings.TrimSpace(line) == "" {
			continue
		}

		// Extract all package names from npm/yarn/pnpm commands
		// Handle multiple packages on same line
		if strings.Contains(line, "npm install") || strings.Contains(line, "npm i") {
			// Remove the command and flags
			cleaned := regexp.MustCompile(`npm\s+i(?:nstall)?\s*(?:-[gDS]\s+|--[a-z-]+\s+)*`).ReplaceAllString(line, "")
			// Split by spaces and extract packages
			parts := strings.Fields(cleaned)
			for _, part := range parts {
				// Remove version specifiers
				pkgName := strings.Split(part, "@")[0]
				if pkgName != "" && !strings.HasPrefix(pkgName, "-") && r.looksLikePackageName(pkgName) {
					packages = append(packages, r.createPackageFromName(pkgName))
				}
				// Handle scoped packages
				if strings.HasPrefix(part, "@") && strings.Contains(part, "/") {
					scopedPkg := regexp.MustCompile(`(@[^/@]+/[^@]+)`).FindStringSubmatch(part)
					if len(scopedPkg) > 1 {
						packages = append(packages, r.createPackageFromName(scopedPkg[1]))
					}
				}
			}
		}

		if strings.Contains(line, "yarn add") || strings.Contains(line, "yarn global add") {
			// Remove the command and flags
			cleaned := regexp.MustCompile(`yarn\s+(?:global\s+)?add\s*(?:--[a-z-]+\s+)*`).ReplaceAllString(line, "")
			// Split by spaces and extract packages
			parts := strings.Fields(cleaned)
			for _, part := range parts {
				// Remove version specifiers
				pkgName := strings.Split(part, "@")[0]
				if pkgName != "" && !strings.HasPrefix(pkgName, "-") && r.looksLikePackageName(pkgName) {
					packages = append(packages, r.createPackageFromName(pkgName))
				}
				// Handle scoped packages
				if strings.HasPrefix(part, "@") && strings.Contains(part, "/") {
					scopedPkg := regexp.MustCompile(`(@[^/@]+/[^@]+)`).FindStringSubmatch(part)
					if len(scopedPkg) > 1 {
						packages = append(packages, r.createPackageFromName(scopedPkg[1]))
					}
				}
			}
		}

		if strings.Contains(line, "pnpm install") || strings.Contains(line, "pnpm add") {
			// Remove the command
			cleaned := regexp.MustCompile(`pnpm\s+(?:install|add)\s*`).ReplaceAllString(line, "")
			// Extract package names
			parts := strings.Fields(cleaned)
			for _, part := range parts {
				if !strings.HasPrefix(part, "-") && r.looksLikePackageName(part) {
					packages = append(packages, r.createPackageFromName(part))
				}
			}
		}

		if strings.Contains(line, "npx ") {
			// Extract npx commands
			npxRegex := regexp.MustCompile(`npx\s+(@?[a-zA-Z0-9/_-]+)`)
			matches := npxRegex.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) > 1 {
					packages = append(packages, r.createPackageFromName(match[1]))
				}
			}
		}

		// Docker RUN commands
		if strings.HasPrefix(strings.TrimSpace(line), "RUN ") {
			runContent := strings.TrimPrefix(strings.TrimSpace(line), "RUN ")
			packages = append(packages, r.extractFromCICD(runContent)...)
		}

		// Makefile targets and commands
		if strings.Contains(line, "$(NPM)") || strings.Contains(line, "$(YARN)") || strings.Contains(line, "$(NPX)") {
			makefileContent := strings.ReplaceAll(line, "$(NPM)", "npm")
			makefileContent = strings.ReplaceAll(makefileContent, "$(YARN)", "yarn")
			makefileContent = strings.ReplaceAll(makefileContent, "$(NPX)", "npx")

			packages = append(packages, r.extractFromCICD(makefileContent)...)
		}
	}

	return packages
}

// extractFromDocumentation extracts packages from documentation files
func (r *Runner) extractFromDocumentation(content string) []Package {
	var packages []Package

	codeBlockRegex := regexp.MustCompile("(?s)```[a-zA-Z]*\n(.*?)\n```")
	codeBlocks := codeBlockRegex.FindAllStringSubmatch(content, -1)
	for _, block := range codeBlocks {
		if len(block) > 1 {
			blockContent := block[1]
			// Extract packages from JavaScript
			packages = append(packages, r.extractFromJavaScript(blockContent)...)
			// Extract from CI/CD commands
			packages = append(packages, r.extractFromCICD(blockContent)...)
		}
	}

	// Extract from inline code and install commands outside code blocks
	contentWithoutBlocks := codeBlockRegex.ReplaceAllString(content, "")

	docPatterns := []*regexp.Regexp{
		regexp.MustCompile(`npm\s+install\s+(?:-[gDS]\s+|--save-dev\s+|--save\s+)?(.+?)(?:\n|$)`),
		regexp.MustCompile(`yarn\s+add\s+(?:--dev\s+)?(.+?)(?:\n|$)`),
		regexp.MustCompile(`pnpm\s+(?:add|install)\s+(.+?)(?:\n|$)`),
		regexp.MustCompile(`npx\s+([a-zA-Z0-9@/_-]+)`),
		regexp.MustCompile(`yarn\s+create\s+([a-zA-Z0-9@/_-]+)`),
		regexp.MustCompile(`npm\s+create\s+([a-zA-Z0-9@/_-]+)`),
	}

	for _, pattern := range docPatterns {
		matches := pattern.FindAllStringSubmatch(contentWithoutBlocks, -1)
		for _, match := range matches {
			if len(match) > 1 {
				parts := strings.Fields(match[1])
				for _, part := range parts {
					// Clean package name
					part = strings.TrimSpace(part)
					part = strings.Trim(part, "`\"'")
					// Remove version specifiers
					if strings.Contains(part, "@") && !strings.HasPrefix(part, "@") {
						part = strings.Split(part, "@")[0]
					}
					if part != "" && !strings.HasPrefix(part, "-") &&
						!r.isBuiltinModule(part) && r.looksLikePackageName(part) {
						packages = append(packages, r.createPackageFromName(part))
					}
				}
			}
		}
	}

	// Extract from inline code backticks
	inlineCodeRegex := regexp.MustCompile("`([a-zA-Z0-9@/_-]+)`")
	inlineMatches := inlineCodeRegex.FindAllStringSubmatch(contentWithoutBlocks, -1)
	for _, match := range inlineMatches {
		if len(match) > 1 && r.looksLikePackageName(match[1]) && !r.isBuiltinModule(match[1]) {
			packages = append(packages, r.createPackageFromName(match[1]))
		}
	}

	// Extract from JSON examples
	jsonExampleRegex := regexp.MustCompile(`"([a-zA-Z0-9@/_-]+)":\s*"[\^~]?[\d.]+.*?"`)
	jsonMatches := jsonExampleRegex.FindAllStringSubmatch(contentWithoutBlocks, -1)
	for _, match := range jsonMatches {
		if len(match) > 1 && r.looksLikePackageName(match[1]) && !r.isBuiltinModule(match[1]) {
			packages = append(packages, r.createPackageFromName(match[1]))
		}
	}

	return packages
}

// looksLikePackageName checks if a string looks like an NPM package name
func (r *Runner) looksLikePackageName(name string) bool {
	if len(name) < 2 {
		return false
	}

	// Skip common words that aren't packages
	commonWords := map[string]bool{
		"name": true, "version": true, "main": true, "test": true, "start": true,
		"build": true, "dev": true, "prod": true, "src": true, "dist": true,
		"lib": true, "bin": true, "scripts": true, "config": true, "index": true,
	}

	if commonWords[strings.ToLower(name)] {
		return false
	}

	// Must contain letters (not just numbers/symbols)
	hasLetters := regexp.MustCompile(`[a-zA-Z]`).MatchString(name)
	return hasLetters
}

// createPackageFromName creates a Package struct from a package name, handling scoped packages
func (r *Runner) createPackageFromName(name string) Package {
	name = strings.TrimSpace(name)

	// Handle scoped packages (@scope/package) - keep the full name for scoped packages
	if strings.HasPrefix(name, "@") && strings.Contains(name, "/") {
		// For scoped packages, keep the full name including @scope/
		return Package{
			Name:      name,
			Namespace: "",
		}
	}

	return Package{
		Name:      name,
		Namespace: "",
	}
}

// isBuiltinModule checks if a module name is a Node.js builtin
func (r *Runner) isBuiltinModule(name string) bool {
	builtins := map[string]bool{
		"fs": true, "path": true, "http": true, "https": true, "url": true,
		"crypto": true, "os": true, "util": true, "events": true, "stream": true,
		"buffer": true, "process": true, "child_process": true, "cluster": true,
		"net": true, "tls": true, "dns": true, "zlib": true, "readline": true,
	}
	return builtins[name]
}

// isPackageClaimed returns true if package is claimed on npmjs.com
func (r *Runner) isPackageClaimed(packageName string) bool {
	url := fmt.Sprintf("https://registry.npmjs.com/%s", packageName)

	resp, err := http.Head(url)
	if err != nil {
		Log.Warningf("Error: %v", err)
		return false
	}

	if resp.StatusCode == http.StatusOK {
		return true
	} else {
		return false
	}
}

// delay returns total delay from options
func (r *Runner) getDelay() time.Duration {
	if r.Options.DelayJitter != 0 {
		return time.Duration(r.Options.Delay + rand.Intn(r.Options.DelayJitter))
	}
	return time.Duration(r.Options.Delay)
}

// hasFileExtension returns true if URL has a file extension
func (r *Runner) hasFileExtension(urlString string) bool {
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		Log.Warningf("Error parsing URL: %v", err)
		return false
	}

	path := parsedURL.Path
	lastSegment := path[strings.LastIndex(path, "/")+1:]
	return strings.Contains(lastSegment, ".")
}

// URL normalization flag rules
const normalizationFlags purell.NormalizationFlags = purell.FlagRemoveDefaultPort |
	purell.FlagLowercaseScheme |
	purell.FlagLowercaseHost |
	purell.FlagDecodeDWORDHost |
	purell.FlagDecodeOctalHost |
	purell.FlagDecodeHexHost |
	purell.FlagRemoveUnnecessaryHostDots |
	purell.FlagRemoveDotSegments |
	purell.FlagRemoveDuplicateSlashes |
	purell.FlagUppercaseEscapes |
	purell.FlagRemoveEmptyPortSeparator |
	purell.FlagDecodeUnnecessaryEscapes |
	purell.FlagRemoveTrailingSlash |
	purell.FlagEncodeNecessaryEscapes |
	purell.FlagSortQuery

// normalizeURLString normalizes a URL string
func normalizeURLString(rawURL string) (normalizedURL string, err error) {
	normalizedURL, err = purell.NormalizeURLString(rawURL, normalizationFlags)
	return normalizedURL, err
}

// hasValidExtension returns true if URL has an extension we can process
func (r *Runner) hasValidExtension(url string) bool {
	// Check JavaScript extensions
	for _, ext := range jsExtensions {
		if strings.HasSuffix(url, ext) {
			return true
		}
	}

	// Check JSON extensions or specific filenames
	if r.isJSONFile(url) {
		return true
	}

	// Check config file extensions
	if r.isConfigFile(url) {
		return true
	}

	// Check CI/CD file extensions
	if r.isCICDFile(url) {
		return true
	}

	// Check documentation file extensions
	if r.isDocFile(url) {
		return true
	}

	return false
}

// trimURLParams removes query params from URL
func trimURLParams(url string) string {
	if strings.Contains(url, "?") {
		return strings.Split(url, "?")[0]
	}
	return url
}

// SetLogLevel sets the logger level
func SetLogLevel(options *Options) {
	Log.Debugln("Setting logger level...")

	if options.Verbose {
		Log.SetLevel(relog.DebugLevel)
	} else if options.Silence {
		Log.SetLevel(relog.FatalLevel)
	} else {
		Log.SetLevel(relog.InfoLevel)
	}
}
