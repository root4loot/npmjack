package runner

import (
	"context"
	"crypto/tls"
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

var (
	regex        = regexp.MustCompile(`\b(?:require|import) ?\(?['"]((@[[a-z\d-]+)?\/?([a-z\d-]+))['"]\)?`)
	jsExtensions = []string{
		".js",
		".mjs",
		".cjs",
		".jsx",
		".ts",
		".tsx",
		".vue",
		".html",
		".htm"}
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
			if r.hasFileExtension(url) && !hasJSExtension(url) {
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

	matches := regex.FindAllStringSubmatch(string(body), -1)

	for _, match := range matches {
		namespace := match[2]
		packageName := match[3]

		if r.isPackageClaimed(packageName) {
			if !seenPackages[packageName] {
				res.Packages = append(res.Packages, Package{Namespace: namespace, Name: packageName, Claimed: true})
				seenPackages[packageName] = true
			}
		} else {
			res.Packages = append(res.Packages, Package{Namespace: namespace, Name: packageName})
		}
	}

	return res
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

// hasJSExtension returns true if URL has an extension that is likely to be a JS file
func hasJSExtension(url string) bool {
	for _, ext := range jsExtensions {
		if strings.HasSuffix(url, ext) {
			return true
		}
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
