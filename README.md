![Go version](https://img.shields.io/badge/Go-v1.19-blue.svg) [![Contribute](https://img.shields.io/badge/Contribute-Welcome-green.svg)](CONTRIBUTING.md)

# npmjack
npmjack is a command-line tool that allows you to find NPM packages from URL's and determine whether they are claimed (public) or not. It can be used to identify and prevent dependency confusion, a type of attack where a public package is replaced with a  malicious one.  

Using npmjack is easy. Simply provide a list of URL's to scan, and npmjack will search for any imported NPM packages in the JavaScript code of those pages. It will then check whether those packages are claimed or not by querying the NPM registry.  

npmjack is a valuable tool for developers who want to ensure that their projects are not vulnerable to dependency confusion attacks. It is also useful for security professionals who need to audit their organization's use of third-party packages.

## Installation

### Go
```
go install github.com/root4loot/npmjack@master
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
   -u,   --url      target URL
   -i,   --infile   file containing targets

CONFIGURATIONS:
   -c,  --concurrency    number of concurrent requests       (Default: 10)
   -t,  --timeout        max request timeout                 (Default: 30 seconds)
   -d,  --delay          delay between requests              (Default: 0 milliseconds)
   -r,  --resolvers      file containing list of resolvers   (Default: System DNS)
   -dj, --delay-jitter   max jitter between requests         (Default: 0 milliseconds)
   -ua, --user-agent     set user agent                      (Default: npmjack)

OUTPUT:
   -o,  --outfile        output results to given file
   -hc, --hide-claimed   hide packages that are claimed
   -s,  --silence        silence everything
   -v,  --verbose        verbose output
        --version        display version
```

## Example

**Single target**
```
$ npmjack -u https://www.hackerone.com/sites/default/files/js/js_C-5Xm0bH3IRZtqPDWPr8Ga4sby1ARHgF6iBlpL4UHao.js
```

**Multiple targets**
```
$ npmjack -i urls.txt
```

**Stream targets (e.g. from [urlwalk](https://github.com/root4loot/urlwalk))**
```
$ urlwalk -t hackerone.com --hide-status --hide-warning | npmjack
```

## Output

```
$ npmjack -i urls.txt

PACKAGE                    NAMESPACE            CLAIMED     SOURCE
-------                    ---------            -------     ------
jquery                                          No          https://www.hackerone.com/sites/default/files/js/js_C-5Xm0bH3IRZtqPDWPr8Ga4sby1ARHgF6iBlpL4UHao.js
vertx                                           No          https://www.hackerone.com/sites/default/files/js/js_4FuDbOJrjJz7g2Uu2GQ6ZFtnbdPymNgBpNtoRkgooH8.js
jquery                                          No          https://www.hackerone.com/sites/default/files/js/js_4FuDbOJrjJz7g2Uu2GQ6ZFtnbdPymNgBpNtoRkgooH8.js
ev-emitter                                      No          https://www.hackerone.com/sites/default/files/js/js_4FuDbOJrjJz7g2Uu2GQ6ZFtnbdPymNgBpNtoRkgooH8.js
```

## As lib

```
go get github.com/root4loot/npmjack
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
