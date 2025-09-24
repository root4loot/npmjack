package runner

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/purell"
	"github.com/root4loot/goutils/hostutil"
	"github.com/root4loot/relog"
)

var Log = relog.NewLogger("npmjack")

type Runner struct {
	Options     Options         // options for the runner
	client      *http.Client    // http client
	resolver    *CustomResolver // custom DNS resolver
	Results     chan Result     // channel to receive results
	Visited     map[string]bool // map of visited urls
	lastResolver string         // last resolver used for tracking
}

type Result struct {
	RequestURL string    // url that was requested
	StatusCode int       // status code of the response
	Packages   []Package // packages found in the response
	Resolver   string    // DNS resolver used for this request
	Error      error     // error if any
}

type Package struct {
	Name      string // package name
	Namespace string // package namespace
	Claimed   bool   // whether the package is claimed or not
}

type Results struct {
	Results []Result
}

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

type SourceMap struct {
	Sources        []string `json:"sources"`
	SourcesContent []string `json:"sourcesContent"`
}

var (
	jsImportRegex         = regexp.MustCompile(`\b(?:require|import)\s*\(?['"]([^'"]+)['"]\)?`)
	jsImportFromRegex     = regexp.MustCompile(`\bfrom\s+['"]([^'"]+)['"]`)
	jsRequireResolveRegex = regexp.MustCompile(`\brequire\.resolve\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	jsDynamicImportRegex  = regexp.MustCompile(`\bimport\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	amdDefineRegex        = regexp.MustCompile(`\bdefine\s*\(\s*\[([^\]]+)\]`)
	amdRequireRegex       = regexp.MustCompile(`\brequire\s*\(\s*\[([^\]]+)\]`)

	npmInstallRegex = regexp.MustCompile(`npm\s+install\s+([a-zA-Z0-9@/_-]+)`)
	yarnAddRegex    = regexp.MustCompile(`yarn\s+add\s+([a-zA-Z0-9@/_-]+)`)

	stringLiteralRegex = regexp.MustCompile(`['"](@?[a-zA-Z0-9/_-]+(?:@[a-zA-Z0-9/_-]+)?/[a-zA-Z0-9/_-]+|[a-zA-Z0-9-]+(?:-[a-zA-Z0-9]+)*)['"]:?`)
	loaderRegex        = regexp.MustCompile(`loader:\s*['"]([^'"]+)['"]`)
	useArrayRegex      = regexp.MustCompile(`use:\s*\[([^\]]+)\]`)
	presetPluginRegex  = regexp.MustCompile(`(?:presets?|plugins?):\s*\[([^\]]+)\]`)

	cdnURLRegex     = regexp.MustCompile(`(?:https?://)?(?:unpkg\.com|cdn\.jsdelivr\.net|cdnjs\.cloudflare\.com)/(?:(?:npm|ajax/libs)/)?(@?[a-zA-Z0-9/_.-]+)`)
	scriptSrcRegex  = regexp.MustCompile(`<script[^>]+src=['"]([^'"]+)['"]`)
	importMapRegex  = regexp.MustCompile(`"(@?[a-zA-Z0-9/_-]+)":\s*['"]https?://[^'"]+['"]`)
	sourceMapNodeModulesRegex = regexp.MustCompile(`webpack://[^/]*/(\.?/)?node_modules/(@?[^/]+(?:/[^/@]+)?)`)
	sourceMapPackageRegex     = regexp.MustCompile(`/(@?[a-zA-Z0-9/_.-]+(?:/[a-zA-Z0-9/_.-]+)?)/`)
	sourceMapFileRegex        = regexp.MustCompile(`node_modules/(@?[a-zA-Z0-9/_.-]+)/`)

	webpackChunkRegex = regexp.MustCompile(`/\*\*\* WEBPACK CHUNK: (@?[a-zA-Z0-9/_-]+) \*\*\*/`)
	bundleCommentRegex = regexp.MustCompile(`/\*[^*]*(@?[a-zA-Z0-9/_-]+(?:/[a-zA-Z0-9/_-]+)?)[^*]*\*/`)

	umdGlobalRegex     = regexp.MustCompile(`(?:window|global)\[?['"](@?[a-zA-Z0-9/_-]+)['"]?\]?\s*=`)
	umdFactoryRegex    = regexp.MustCompile(`factory\s*\(\s*(?:require\s*\(\s*['"]([^'"]+)['"]|(['"][^'"]+['"]))`)
	globalAssignRegex  = regexp.MustCompile(`(?:window|global)\.(@?[A-Za-z][A-Za-z0-9_$]*)\s*=`)

	minifiedCallRegex    = regexp.MustCompile(`\b[a-z]\(['"](@?[a-zA-Z0-9/_-]+)['"]`)
	minifiedRequireRegex = regexp.MustCompile(`\b(?:n|r|e)\((\d+)\)`)
	parcelRequireRegex   = regexp.MustCompile(`parcel\$require\(['"]([^'"]+)['"]\)`)

	webpackExternalRegex = regexp.MustCompile(`externals\s*:\s*\{([^}]+)\}`)
	rollupBundleRegex    = regexp.MustCompile(`// rollup bundle.*?require\(['"]([^'"]+)['"]\)`)

	jsonExtensions      = []string{".json"}
	cicdExtensions      = []string{".yml", ".yaml", ".sh", ".bash"}
	docExtensions       = []string{".md", ".rst", ".txt"}
	sourceMapExtensions = []string{".map"}

	excludedExtensions = []string{
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".tif", ".webp", ".svg", ".ico", ".cur",
		".psd", ".ai", ".eps", ".raw", ".cr2", ".nef", ".orf", ".sr2", ".dng",
		".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", ".mkv", ".m4v", ".3gp", ".ogv",
		".mpg", ".mpeg", ".m2v", ".m4p", ".m4b", ".f4v", ".f4p", ".f4a", ".f4b",
		".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma", ".m4a", ".opus", ".amr",
		".aiff", ".au", ".ra", ".3ga", ".ac3", ".ape", ".caf", ".dts", ".m4r", ".mka", ".tak", ".tta", ".wv",
		".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".lz", ".lzma", ".Z", ".cab", ".arj", ".lha", ".ace", ".zoo", ".arc", ".pak", ".pit", ".sit", ".sitx", ".sea", ".hqx",
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".odt", ".ods", ".odp",
		".rtf", ".pages", ".numbers", ".key",
		".exe", ".msi", ".deb", ".rpm", ".dmg", ".pkg", ".app", ".run", ".bin", ".com", ".scr",
		".bat", ".cmd", ".ps1", ".vbs", ".jar", ".war", ".ear",
		".ttf", ".otf", ".woff", ".woff2", ".eot", ".fon", ".fnt",
		".db", ".sqlite", ".sqlite3", ".mdb", ".accdb", ".dbf",
		".iso", ".img", ".vdi", ".vmdk", ".vhd",
		".dll", ".so", ".dylib", ".lib", ".a", ".o", ".obj",
		".swf", ".fla", ".as", ".class",
	}
)

type Options struct {
	Concurrency int
	Timeout     int
	Delay       int
	DelayJitter int
	Verbose     bool
	Silence     bool
	UserAgent   string
	Proxy       string
	Resolvers   []string
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

// NewRunner creates a new package scanner
func NewRunner() *Runner {
	options := DefaultOptions()
	SetLogLevel(options)

	resolver := NewCustomResolver([]string{}, time.Duration(options.Timeout)*time.Second)

	transport := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		MaxIdleConnsPerHost:   options.Concurrency,
		ResponseHeaderTimeout: time.Duration(options.Timeout) * time.Second,
		DialContext:           resolver.CustomDialContext,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(options.Timeout) * time.Second,
	}

	return &Runner{
		Results:  make(chan Result),
		Visited:  make(map[string]bool),
		Options:  *options,
		client:   client,
		resolver: resolver,
	}
}

func (r *Runner) Run(urls ...string) {
	defer close(r.Results)

	if len(r.Options.Resolvers) > 0 {
		r.resolver = NewCustomResolver(r.Options.Resolvers, time.Duration(r.Options.Timeout)*time.Second)
		if transport, ok := r.client.Transport.(*http.Transport); ok {
			transport.DialContext = r.resolver.CustomDialContext
		}
	}

	if r.Options.Proxy != "" {
		if transport, ok := r.client.Transport.(*http.Transport); ok {
			if !hostutil.IsValidHostWithPort(r.Options.Proxy) {
				Log.Warningf("Invalid proxy format (expected host:port): %s", r.Options.Proxy)
			} else {
				proxyURL := &url.URL{
					Scheme: "http",
					Host:   r.Options.Proxy,
				}
				transport.Proxy = http.ProxyURL(proxyURL)
			}
		}
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
	res := Result{
		RequestURL: url,
		StatusCode: resp.StatusCode,
		Resolver:   r.getLastResolver(),
		Error:      err,
	}

	seenPackages := make(map[string]bool)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		Log.Warningf("Error reading response body: %v", err)
		return Result{RequestURL: url, Error: err}
	}

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

func (r *Runner) extractPackages(url, content string) []Package {
	var packages []Package

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

	if r.isSourceMapFile(url) {
		packages = append(packages, r.extractFromSourceMap(content)...)
	}

	packages = append(packages, r.extractFromJavaScript(content)...)

	return packages
}

func (r *Runner) isJSONFile(url string) bool {
	for _, ext := range jsonExtensions {
		if strings.HasSuffix(url, ext) {
			return true
		}
	}
	lowerURL := strings.ToLower(url)
	return strings.Contains(lowerURL, "package.json") ||
		strings.Contains(lowerURL, "package-lock.json") ||
		strings.Contains(lowerURL, "yarn.lock")
}

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

func (r *Runner) isDocFile(url string) bool {
	for _, ext := range docExtensions {
		if strings.HasSuffix(strings.ToLower(url), ext) {
			return true
		}
	}
	return false
}

func (r *Runner) isSourceMapFile(url string) bool {
	for _, ext := range sourceMapExtensions {
		if strings.HasSuffix(strings.ToLower(url), ext) {
			return true
		}
	}
	return false
}

func (r *Runner) extractFromJSON(content string) []Package {
	var packages []Package

	if pkgJSON := r.parsePackageJSON(content); pkgJSON != nil {
		packages = append(packages, r.extractFromPackageJSON(pkgJSON)...)
	}

	if lockJSON := r.parsePackageLockJSON(content); lockJSON != nil {
		packages = append(packages, r.extractFromPackageLockJSON(lockJSON)...)
	}

	// yarn.lock
	packages = append(packages, r.extractFromYarnLock(content)...)

	return packages
}

func (r *Runner) parsePackageJSON(content string) *PackageJSON {
	var pkg PackageJSON
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return nil
	}
	return &pkg
}

func (r *Runner) parsePackageLockJSON(content string) *PackageLockJSON {
	var lockFile PackageLockJSON
	if err := json.Unmarshal([]byte(content), &lockFile); err != nil {
		return nil
	}
	return &lockFile
}

func (r *Runner) extractFromPackageJSON(pkg *PackageJSON) []Package {
	var packages []Package

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

	for _, name := range pkg.BundledDependencies {
		packages = append(packages, r.createPackageFromName(name))
	}

	return packages
}

func (r *Runner) extractFromPackageLockJSON(lockFile *PackageLockJSON) []Package {
	var packages []Package

	for packagePath, entry := range lockFile.Packages {
		if strings.Contains(packagePath, "node_modules/") {
			parts := strings.Split(packagePath, "node_modules/")
			if len(parts) > 1 {
				name := parts[len(parts)-1]
				packages = append(packages, r.createPackageFromName(name))
			}
		}

		for depName := range entry.Dependencies {
			packages = append(packages, r.createPackageFromName(depName))
		}
	}

	return packages
}

func (r *Runner) extractFromYarnLock(content string) []Package {
	var packages []Package

	scopedPackageRegex := regexp.MustCompile(`^"(@[^/]+/[^@"]+)(?:@[^"]*)?":`)
	normalPackageRegex := regexp.MustCompile(`^"([^@"]+)(?:@[^"]*)?":`)
	dependencyRegex := regexp.MustCompile(`^\s{4}"?(@?[a-zA-Z0-9/@_-]+)"?\s+"([^"]+)"`)

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		originalLine := line
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// scoped packages
		if matches := scopedPackageRegex.FindStringSubmatch(line); matches != nil {
			name := matches[1]
			packages = append(packages, r.createPackageFromName(name))
			continue
		}

		// normal packages
		if matches := normalPackageRegex.FindStringSubmatch(line); matches != nil {
			name := matches[1]
			packages = append(packages, r.createPackageFromName(name))
			continue
		}

		// dependencies
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

func (r *Runner) extractFromSourceMap(content string) []Package {
	var packages []Package

	var sourceMap SourceMap
	if err := json.Unmarshal([]byte(content), &sourceMap); err != nil {
		return packages
	}

	for _, source := range sourceMap.Sources {
		if matches := sourceMapNodeModulesRegex.FindAllStringSubmatch(source, -1); matches != nil {
			for _, match := range matches {
				if len(match) > 2 {
					pkgName := match[2]
					if !r.isBuiltinModule(pkgName) && r.looksLikePackageName(pkgName) {
						packages = append(packages, r.createPackageFromName(pkgName))
					}
				}
			}
		}

		if matches := sourceMapPackageRegex.FindAllStringSubmatch(source, -1); matches != nil {
			for _, match := range matches {
				if len(match) > 1 {
					pkgName := match[1]
					if !r.isBuiltinModule(pkgName) && r.looksLikePackageName(pkgName) {
						packages = append(packages, r.createPackageFromName(pkgName))
					}
				}
			}
		}

		if matches := sourceMapFileRegex.FindAllStringSubmatch(source, -1); matches != nil {
			for _, match := range matches {
				if len(match) > 1 {
					pkgName := match[1]
					if !r.isBuiltinModule(pkgName) && r.looksLikePackageName(pkgName) {
						packages = append(packages, r.createPackageFromName(pkgName))
					}
				}
			}
		}
	}

	for _, sourceContent := range sourceMap.SourcesContent {
		if sourceContent != "" {
			packages = append(packages, r.extractFromJavaScript(sourceContent)...)
		}
	}

	return packages
}

func (r *Runner) extractFromJavaScript(content string) []Package {
	var packages []Package

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

	amdDefineMatches := amdDefineRegex.FindAllStringSubmatch(content, -1)
	for _, match := range amdDefineMatches {
		if len(match) > 1 {
			deps := r.parseAMDDependencies(match[1])
			for _, dep := range deps {
				packages = append(packages, r.createPackageFromName(dep))
			}
		}
	}

	amdRequireMatches := amdRequireRegex.FindAllStringSubmatch(content, -1)
	for _, match := range amdRequireMatches {
		if len(match) > 1 {
			deps := r.parseAMDDependencies(match[1])
			for _, dep := range deps {
				packages = append(packages, r.createPackageFromName(dep))
			}
		}
	}

	requireConfigRegex := regexp.MustCompile(`paths:\s*\{([^}]+)\}`)
	configMatches := requireConfigRegex.FindAllStringSubmatch(content, -1)
	for _, match := range configMatches {
		if len(match) > 1 {
			pathRegex := regexp.MustCompile(`['"]([^'"]+)['"]\s*:\s*['"][^'"]+['"]`)
			pathMatches := pathRegex.FindAllStringSubmatch(match[1], -1)
			for _, pm := range pathMatches {
				if len(pm) > 1 && !r.isBuiltinModule(pm[1]) {
					packages = append(packages, r.createPackageFromName(pm[1]))
				}
			}
		}
	}

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

	packages = append(packages, r.extractFromCDNUrls(content)...)
	packages = append(packages, r.extractFromBundleComments(content)...)
	packages = append(packages, r.extractFromUMDPatterns(content)...)
	packages = append(packages, r.extractFromMinifiedCode(content)...)
	packages = append(packages, r.extractFromWebpackExternals(content)...)

	return packages
}

func (r *Runner) extractFromCDNUrls(content string) []Package {
	var packages []Package

	scriptMatches := scriptSrcRegex.FindAllStringSubmatch(content, -1)
	for _, match := range scriptMatches {
		if len(match) > 1 {
			packages = append(packages, r.extractPackagesFromURL(match[1])...)
		}
	}

	cdnMatches := cdnURLRegex.FindAllStringSubmatch(content, -1)
	for _, match := range cdnMatches {
		if len(match) > 1 {
			pkgName := match[1]
			if !r.isBuiltinModule(pkgName) && r.looksLikePackageName(pkgName) {
				packages = append(packages, r.createPackageFromName(pkgName))
			}
		}
	}

	importMapMatches := importMapRegex.FindAllStringSubmatch(content, -1)
	for _, match := range importMapMatches {
		if len(match) > 1 {
			pkgName := match[1]
			if !r.isBuiltinModule(pkgName) && r.looksLikePackageName(pkgName) {
				packages = append(packages, r.createPackageFromName(pkgName))
			}
		}
	}

	return packages
}

func (r *Runner) extractPackagesFromURL(url string) []Package {
	var packages []Package

	matches := cdnURLRegex.FindAllStringSubmatch(url, -1)
	for _, match := range matches {
		if len(match) > 1 {
			pkgName := match[1]
			if !r.isBuiltinModule(pkgName) && r.looksLikePackageName(pkgName) {
				packages = append(packages, r.createPackageFromName(pkgName))
			}
		}
	}

	return packages
}

func (r *Runner) extractFromBundleComments(content string) []Package {
	var packages []Package

	chunkMatches := webpackChunkRegex.FindAllStringSubmatch(content, -1)
	for _, match := range chunkMatches {
		if len(match) > 1 {
			pkgName := match[1]
			if !r.isBuiltinModule(pkgName) && r.looksLikePackageName(pkgName) {
				packages = append(packages, r.createPackageFromName(pkgName))
			}
		}
	}

	commentMatches := bundleCommentRegex.FindAllStringSubmatch(content, -1)
	for _, match := range commentMatches {
		if len(match) > 1 {
			pkgName := match[1]
			if !r.isBuiltinModule(pkgName) && r.looksLikePackageName(pkgName) {
				packages = append(packages, r.createPackageFromName(pkgName))
			}
		}
	}

	return packages
}

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

func (r *Runner) extractFromUMDPatterns(content string) []Package {
	var packages []Package

	globalMatches := umdGlobalRegex.FindAllStringSubmatch(content, -1)
	for _, match := range globalMatches {
		if len(match) > 1 {
			pkgName := match[1]
			if !r.isBuiltinModule(pkgName) && r.looksLikePackageName(pkgName) {
				packages = append(packages, r.createPackageFromName(pkgName))
			}
		}
	}

	factoryMatches := umdFactoryRegex.FindAllStringSubmatch(content, -1)
	for _, match := range factoryMatches {
		for i := 1; i < len(match); i++ {
			if match[i] != "" {
				pkgName := strings.Trim(match[i], `'"`)
				if !r.isBuiltinModule(pkgName) && r.looksLikePackageName(pkgName) {
					packages = append(packages, r.createPackageFromName(pkgName))
				}
			}
		}
	}

	assignMatches := globalAssignRegex.FindAllStringSubmatch(content, -1)
	for _, match := range assignMatches {
		if len(match) > 1 {
			pkgName := strings.ToLower(match[1])
			if !r.isBuiltinModule(pkgName) && r.looksLikePackageName(pkgName) {
				packages = append(packages, r.createPackageFromName(pkgName))
			}
		}
	}

	return packages
}

func (r *Runner) extractFromMinifiedCode(content string) []Package {
	var packages []Package

	callMatches := minifiedCallRegex.FindAllStringSubmatch(content, -1)
	for _, match := range callMatches {
		if len(match) > 1 {
			pkgName := match[1]
			if !r.isBuiltinModule(pkgName) && r.looksLikePackageName(pkgName) {
				packages = append(packages, r.createPackageFromName(pkgName))
			}
		}
	}

	parcelMatches := parcelRequireRegex.FindAllStringSubmatch(content, -1)
	for _, match := range parcelMatches {
		if len(match) > 1 {
			pkgName := match[1]
			if !r.isBuiltinModule(pkgName) && r.looksLikePackageName(pkgName) {
				packages = append(packages, r.createPackageFromName(pkgName))
			}
		}
	}

	rollupMatches := rollupBundleRegex.FindAllStringSubmatch(content, -1)
	for _, match := range rollupMatches {
		if len(match) > 1 {
			pkgName := match[1]
			if !r.isBuiltinModule(pkgName) && r.looksLikePackageName(pkgName) {
				packages = append(packages, r.createPackageFromName(pkgName))
			}
		}
	}

	return packages
}

func (r *Runner) extractFromWebpackExternals(content string) []Package {
	var packages []Package

	externalMatches := webpackExternalRegex.FindAllStringSubmatch(content, -1)
	for _, match := range externalMatches {
		if len(match) > 1 {
			externalsBlock := match[1]
			keyRegex := regexp.MustCompile(`['"](@?[a-zA-Z0-9/_-]+)['"]`)
			keyMatches := keyRegex.FindAllStringSubmatch(externalsBlock, -1)
			for _, km := range keyMatches {
				if len(km) > 1 {
					pkgName := km[1]
					if !r.isBuiltinModule(pkgName) && r.looksLikePackageName(pkgName) {
						packages = append(packages, r.createPackageFromName(pkgName))
					}
				}
			}
		}
	}

	return packages
}

func (r *Runner) extractFromConfigFile(content string) []Package {
	var packages []Package

	if strings.Contains(content, "{") && strings.Contains(content, "}") {
		packages = append(packages, r.extractFromJSON(content)...)
	}

	packages = append(packages, r.extractFromJavaScript(content)...)

	configPatterns := []*regexp.Regexp{
		loaderRegex,
		stringLiteralRegex,
	}

	for _, pattern := range configPatterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				name := match[1]
				name = strings.Trim(name, `'"`)
				if r.looksLikePackageName(name) && !r.isBuiltinModule(name) {
					packages = append(packages, r.createPackageFromName(name))
				}
			}
		}
	}

	arrayPatterns := []*regexp.Regexp{
		useArrayRegex,
		presetPluginRegex,
	}

	for _, pattern := range arrayPatterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				arrayContent := match[1]
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

func (r *Runner) extractFromCICD(content string) []Package {
	var packages []Package

	lines := strings.Split(content, "\n")

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") || strings.TrimSpace(line) == "" {
			continue
		}

		if strings.Contains(line, "npm install") || strings.Contains(line, "npm i") {
			cleaned := regexp.MustCompile(`npm\s+i(?:nstall)?\s*(?:-[gDS]\s+|--[a-z-]+\s+)*`).ReplaceAllString(line, "")
			parts := strings.Fields(cleaned)
			for _, part := range parts {
				pkgName := strings.Split(part, "@")[0]
				if pkgName != "" && !strings.HasPrefix(pkgName, "-") && r.looksLikePackageName(pkgName) {
					packages = append(packages, r.createPackageFromName(pkgName))
				}
				if strings.HasPrefix(part, "@") && strings.Contains(part, "/") {
					scopedPkg := regexp.MustCompile(`(@[^/@]+/[^@]+)`).FindStringSubmatch(part)
					if len(scopedPkg) > 1 {
						packages = append(packages, r.createPackageFromName(scopedPkg[1]))
					}
				}
			}
		}

		if strings.Contains(line, "yarn add") || strings.Contains(line, "yarn global add") {
			cleaned := regexp.MustCompile(`yarn\s+(?:global\s+)?add\s*(?:--[a-z-]+\s+)*`).ReplaceAllString(line, "")
			parts := strings.Fields(cleaned)
			for _, part := range parts {
				pkgName := strings.Split(part, "@")[0]
				if pkgName != "" && !strings.HasPrefix(pkgName, "-") && r.looksLikePackageName(pkgName) {
					packages = append(packages, r.createPackageFromName(pkgName))
				}
				if strings.HasPrefix(part, "@") && strings.Contains(part, "/") {
					scopedPkg := regexp.MustCompile(`(@[^/@]+/[^@]+)`).FindStringSubmatch(part)
					if len(scopedPkg) > 1 {
						packages = append(packages, r.createPackageFromName(scopedPkg[1]))
					}
				}
			}
		}

		if strings.Contains(line, "pnpm install") || strings.Contains(line, "pnpm add") {
			cleaned := regexp.MustCompile(`pnpm\s+(?:install|add)\s*`).ReplaceAllString(line, "")
			parts := strings.Fields(cleaned)
			for _, part := range parts {
				if !strings.HasPrefix(part, "-") && r.looksLikePackageName(part) {
					packages = append(packages, r.createPackageFromName(part))
				}
			}
		}

		if strings.Contains(line, "npx ") {
			npxRegex := regexp.MustCompile(`npx\s+(@?[a-zA-Z0-9/_-]+)`)
			matches := npxRegex.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) > 1 {
					packages = append(packages, r.createPackageFromName(match[1]))
				}
			}
		}

		if strings.HasPrefix(strings.TrimSpace(line), "RUN ") {
			runContent := strings.TrimPrefix(strings.TrimSpace(line), "RUN ")
			packages = append(packages, r.extractFromCICD(runContent)...)
		}

		if strings.Contains(line, "$(NPM)") || strings.Contains(line, "$(YARN)") || strings.Contains(line, "$(NPX)") {
			makefileContent := strings.ReplaceAll(line, "$(NPM)", "npm")
			makefileContent = strings.ReplaceAll(makefileContent, "$(YARN)", "yarn")
			makefileContent = strings.ReplaceAll(makefileContent, "$(NPX)", "npx")

			packages = append(packages, r.extractFromCICD(makefileContent)...)
		}
	}

	return packages
}

func (r *Runner) extractFromDocumentation(content string) []Package {
	var packages []Package

	codeBlockRegex := regexp.MustCompile("(?s)```[a-zA-Z]*\n(.*?)\n```")
	codeBlocks := codeBlockRegex.FindAllStringSubmatch(content, -1)
	for _, block := range codeBlocks {
		if len(block) > 1 {
			blockContent := block[1]
			packages = append(packages, r.extractFromJavaScript(blockContent)...)
			packages = append(packages, r.extractFromCICD(blockContent)...)
		}
	}

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
					part = strings.TrimSpace(part)
					part = strings.Trim(part, "`\"'")
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

	inlineCodeRegex := regexp.MustCompile("`([a-zA-Z0-9@/_-]+)`")
	inlineMatches := inlineCodeRegex.FindAllStringSubmatch(contentWithoutBlocks, -1)
	for _, match := range inlineMatches {
		if len(match) > 1 && r.looksLikePackageName(match[1]) && !r.isBuiltinModule(match[1]) {
			packages = append(packages, r.createPackageFromName(match[1]))
		}
	}

	jsonExampleRegex := regexp.MustCompile(`"([a-zA-Z0-9@/_-]+)":\s*"[\^~]?[\d.]+.*?"`)
	jsonMatches := jsonExampleRegex.FindAllStringSubmatch(contentWithoutBlocks, -1)
	for _, match := range jsonMatches {
		if len(match) > 1 && r.looksLikePackageName(match[1]) && !r.isBuiltinModule(match[1]) {
			packages = append(packages, r.createPackageFromName(match[1]))
		}
	}

	return packages
}

func (r *Runner) looksLikePackageName(name string) bool {
	if len(name) < 2 {
		return false
	}

	commonWords := map[string]bool{
		"name": true, "version": true, "main": true, "test": true, "start": true,
		"build": true, "dev": true, "prod": true, "src": true, "dist": true,
		"lib": true, "bin": true, "scripts": true, "config": true, "index": true,
	}

	if commonWords[strings.ToLower(name)] {
		return false
	}

	hasLetters := regexp.MustCompile(`[a-zA-Z]`).MatchString(name)
	return hasLetters
}

func (r *Runner) createPackageFromName(name string) Package {
	name = strings.TrimSpace(name)

	if strings.HasPrefix(name, "@") && strings.Contains(name, "/") {
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

func (r *Runner) isBuiltinModule(name string) bool {
	builtins := map[string]bool{
		"fs": true, "path": true, "http": true, "https": true, "url": true,
		"crypto": true, "os": true, "util": true, "events": true, "stream": true,
		"buffer": true, "process": true, "child_process": true, "cluster": true,
		"net": true, "tls": true, "dns": true, "zlib": true, "readline": true,
	}
	return builtins[name]
}

func (r *Runner) isPackageClaimed(packageName string) bool {
	url := fmt.Sprintf("https://registry.npmjs.com/%s", packageName)

	resp, err := r.client.Head(url)
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

func (r *Runner) getDelay() time.Duration {
	if r.Options.DelayJitter != 0 {
		return time.Duration(r.Options.Delay + rand.Intn(r.Options.DelayJitter))
	}
	return time.Duration(r.Options.Delay)
}

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

func normalizeURLString(rawURL string) (normalizedURL string, err error) {
	normalizedURL, err = purell.NormalizeURLString(rawURL, normalizationFlags)
	return normalizedURL, err
}

func (r *Runner) hasValidExtension(url string) bool {
	for _, ext := range excludedExtensions {
		if strings.HasSuffix(strings.ToLower(url), ext) {
			return false
		}
	}
	return true
}

func trimURLParams(url string) string {
	if strings.Contains(url, "?") {
		return strings.Split(url, "?")[0]
	}
	return url
}

func (r *Runner) getLastResolver() string {
	if r.resolver != nil {
		return r.resolver.lastResolver
	}
	return "system"
}


// SetLogLevel configures logger verbosity
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
