package main

import (
	"fmt"

	npmjack "github.com/root4loot/npmjack/pkg/runner"
	options "github.com/root4loot/recrawl/pkg/options"
	recrawl "github.com/root4loot/recrawl/pkg/runner"
)

func main() {
	recrawlOptions := options.Options{
		Concurrency: 20,
		Timeout:     10,
		Resolvers:   []string{"8.8.8.8", "208.67.222.222"},
	}

	// initialize recrawl and npmjack
	recrawl := recrawl.NewRunner(&recrawlOptions)
	npmjack := npmjack.NewRunner()

	// process results from recrawl
	go func() {
		for result := range recrawl.Results {
			if result.StatusCode == 200 {
				npmjack.Run(result.RequestURL)
			}
		}
	}()

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

	// grab urls with recrawl
	recrawl.Run("hackerone.com")
}
