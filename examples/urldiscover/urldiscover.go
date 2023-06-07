package main

import (
	"fmt"

	npmjack "github.com/root4loot/npmjack/pkg/runner"
	options "github.com/root4loot/urlwalk/pkg/options"
	urlwalk "github.com/root4loot/urlwalk/pkg/runner"
)

func main() {
	urlwalkOptions := options.Options{
		Concurrency: 20,
		Timeout:     10,
		Resolvers:   []string{"8.8.8.8", "208.67.222.222"},
	}

	// initialize urlwalk and npmjack
	urlwalk := urlwalk.NewRunner(&urlwalkOptions)
	npmjack := npmjack.NewRunner()

	// process results from urlwalk
	go func() {
		for result := range urlwalk.Results {
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

	// grab urls with urlwalk
	urlwalk.Run("hackerone.com")
}
