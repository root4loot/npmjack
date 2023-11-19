<br>
<div align="center">
  <br>
  <img src="assets/logo.png" alt="recrawl logo" width="300">
</div>

<br>

<div align="center">
 A tool used to scan JavaScript files for NPM packages and assess their claimability. Handy for spotting Dependency Confusion vulnerabilities.
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

Use [recrawl](https://github.com/root4loot/recrawl) to find `.js` URLs and pipe its results to NpmJack

```sh
recrawl -t hackerone.com --hide-status --hide-warning | npmjack
```

## Output

```sh
$ recrawl -t hackerone.com --hide-status --hide-warning | npmjack

PACKAGE                    NAMESPACE            CLAIMED   SOURCE
-------                    ---------            -------   ------
jquery                                          Yes         https://www.hackerone.com/sites/default/files/js/js_EOrKavGmjAkpIaCW_cpGJ240OpVZev_5NI-WGIx5URg.js
jquery                                          Yes         https://www.hackerone.com/sites/default/files/js/js_ol7H2KkxPxe7E03XeuZQO5qMcg0RpfSOgrm_Kg94rOs.js
jquery                                          Yes         https://www.hackerone.com/sites/default/files/js/js_1yMolXFTeaqGGhfYh1qdP42Cf06oH4PgdG9FhiGwbS8.js
jquery                                          Yes         https://www.hackerone.com/sites/default/files/js/js_xF9mKu6OVNysPMy7w3zYTWNPFBDlury_lEKDCfRuuHs.js
jquery                                          Yes         https://www.hackerone.com/sites/default/files/js/js_coYiv6lRieZN3l0IkRYgmvrMASvFk2BL-jdq5yjFbGs.js
vertx                                           Yes         https://www.hackerone.com/sites/default/files/js/js_49X7xBwrMQ94DmEeXrZsMj2O2H09Jn12bOR4pcENzvU.js
jquery                                          Yes         https://www.hackerone.com/sites/default/files/js/js_49X7xBwrMQ94DmEeXrZsMj2O2H09Jn12bOR4pcENzvU.js
jquery                                          Yes         https://www.hackerone.com/sites/default/files/js/js_4fGl1ylmYP1UN1LYpgag5KeomdCw60f9TrcboP7n_xc.js
sinatra                                         Yes         https://www.hackerone.com/application-security/how-server-side-request-forgery-ssrf
open-uri                                        Yes         https://www.hackerone.com/application-security/how-server-side-request-forgery-ssrf
util                                            Yes         https://hackerone.com/assets/static/js/vendor.fb1db314.js
react-resizable                                 Yes         https://hackerone.com/assets/static/js/vendor.fb1db314.js
jquery                                          Yes         https://www.hackerone.com/sites/default/files/js/js_q5jqDjlruRFH40xInB2iWuzyyIWbybGtXXw_8ZmMm-w.js
jquery                                          Yes         https://www.hackerone.com/sites/default/files/js/js_szq9MnNU-7YXnmbxrcpn4I5JxoF3SYq-k1Gf0mENDIk.js
jquery                                          Yes         https://www.hackerone.com/sites/default/files/js/js_5YhGQsbctK8n_K7tBlFMqnbjvtPLRqOKAF7UOGQibrg.js
jquery                                          Yes         https://www.hackerone.com/sites/default/files/js/js_jnaihVoc8oP0HbDoCX33ERgmAxK93_JCLONQldYU1Co.js
jquery                                          Yes         https://www.hackerone.com/sites/default/files/js/js_MwkUR38zEDMq2cgfwWUm-0QRjnW_3E1DUhoSTqF5cEg.js
jquery                                          Yes         https://www.hackerone.com/sites/default/files/js/js_YVxHw88AWuNDg2_UcWD3YEGdw-OMJOJSCa94-eiftk8.js
vertx                                           Yes         https://www.hackerone.com/sites/default/files/js/js_MrK8-vEN31hvJ3cKuoqF_s1MtpXe7eZC4nwEKAqLALQ.js
jquery                                          Yes         https://www.hackerone.com/sites/default/files/js/js_MrK8-vEN31hvJ3cKuoqF_s1MtpXe7eZC4nwEKAqLALQ.js
jquery                                          Yes         https://www.hackerone.com/sites/default/files/js/js_VhuPXvhVksnz0EKsZaNqchtw6drabbGIMEJFhaLOlx8.js
jquery                                          Yes         https://www.hackerone.com/sites/default/files/js/js_Y2J8iu30we2OrQ1FC9uh739UPsQjLhTsbhsE8_jQ6jg.js
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
