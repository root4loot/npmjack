<br>
<div align="center">
  <br>
  <img src="assets/logo.png" alt="recrawl logo" width="300">
  <br><br>
  <a href="https://github.com/root4loot/npmjack/actions/workflows/build.yml">
    <img src="https://github.com/root4loot/npmjack/actions/workflows/build.yml/badge.svg" alt="Build Status">
  </a>
</div>

<br>

<div align="center">
 A tool to find NPM packages in web files and check if they're claimable. Useful for finding dependency confusion bugs.
</div>

<br>


## Installation

### Go
```
go install github.com/root4loot/npmjack@latest
```

### Docker
```
git clone https://github.com/root4loot/npmjack.git && cd npmjack
docker build -t npmjack .
docker run -it npmjack -h
```


## Usage
```
Usage: ./npmjack [options] (-u <url> | -l <target-list>)

TARGETING:
   -u,  --url            target URL
   -i,  --infile         file containing URL's (newline separated)

CONFIGURATIONS:
   -c,  --concurrency    number of concurrent requests       (Default: 10)
   -t,  --timeout        max request timeout                 (Default: 30 seconds)
   -d,  --delay          delay between requests              (Default: 0 milliseconds)
   -r,  --resolvers      file containing list of resolvers   (Default: System DNS)
   -dj, --delay-jitter   max jitter between requests         (Default: 0 milliseconds)
   -ua, --user-agent     set user agent                      (Default: npmjack)
   -p,  --proxy          proxy URL                           (Example: 127.0.0.1:8080)

OUTPUT:
   -o,  --outfile        output results to given file
   -hc, --hide-claimed   hide packages that are claimed
   -s,  --silence        silence everything
   -v,  --verbose        verbose output
        --version        display version
```

## Example

**Single URL**
```sh
npmjack -u https://www.hackerone.com/sites/default/files/js/js_C-5Xm0bH3IRZtqPDWPr8Ga4sby1ARHgF6iBlpL4UHao.js
```

**Multiple URLs**
```sh
npmjack -i urls.txt
```

Use [recrawl](https://github.com/root4loot/recrawl) to find all URLs and pipe them to npmjack (which filters out supported file types)

```sh
recrawl -t target.com --hide-status --hide-warning | npmjack
```

npmjack detects NPM packages in JS/TypeScript files, configuration files, CI/CD files, and documentation. It handles import/require statements, scoped packages, version specifiers, and build tool configurations.

## Detection Methods

npmjack uses several techniques to find NPM packages in different types of files. It looks through JS and TypeScript code for import and require statements, and checks package.json files and webpack configs. The tool can also parse source maps to find packages in minified code, which helps discover dependencies even when the original code has been compressed or bundled.

For single-page apps, npmjack analyzes bundled JS files to identify module patterns from bundlers like webpack and rollup. It can handle UMD and AMD modules found in older applications, and detects minified libraries by looking for common compression patterns. The tool also finds CDN-hosted packages by checking URL patterns and parses webpack externals to catch packages loaded separately from the main bundle.

## Sample Output

```sh
$ recrawl -t target.com --hide-status --hide-warning | npmjack

PACKAGE                    NAMESPACE            CLAIMED   SOURCE
-------                    ---------            -------   ------
jquery                                          Yes         https://www.target.com/assets/js/app.js
express                                         Yes         https://www.target.com/package.json
@babel/core                @babel/              No          https://www.target.com/webpack.config.js
missing-package                                 No          https://www.target.com/Dockerfile
typescript                                      Yes         https://www.target.com/.github/workflows/ci.yml
```

## As lib

```
go get github.com/root4loot/npmjack@latest
```

```go
package main

import (
	"fmt"

	npmjack "github.com/root4loot/npmjack/pkg/runner"
)

func main() {
	urls := []string{"https://www.hackerone.com/sites/default/files/js/js_Ikd9nsZ0AFAesOLgcgjc7F6CRoODbeqOn7SVbsXgALQ.js",
		"https://www.hackerone.com/sites/default/files/js/js_C-5Xm0bH3IRZtqPDWPr8Ga4sby1ARHgF6iBlpL4UHao.js",
		"https://www.hackerone.com/sites/default/files/js/js_4FuDbOJrjJz7g2Uu2GQ6ZFtnbdPymNgBpNtoRkgooH8.js",
		"https://www.hackerone.com/sites/default/files/js/js_zApVJ5sm-YHSWP4O5K9MqZ_6q4nDR3MciTUC3Pr1ogA.js",
		"https://www.hackerone.com/sites/default/files/js/js_edjgXnk09wjvbZfyK_TkFKU4uhpo1LGgJBnFdeu6aH8.js"}

	// initialize npmjack
	npmjack := npmjack.NewRunner()

	// process results from npmjack
	go func() {
		for result := range npmjack.Results {
			if result.StatusCode == 200 {
				for _, pkg := range result.Packages {
					fmt.Println("Package", pkg.Name, "on", result.RequestURL, "Claimed:", pkg.Claimed)
				}
			}
		}
	}()

	// run npmjack
	for _, url := range urls {
		npmjack.Run(url)
	}
}
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md)
