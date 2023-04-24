package main

import (
	"fmt"

	npmjack "github.com/root4loot/npmjack/pkg/runner"
	options "github.com/root4loot/urldiscover/pkg/options"
	urldiscover "github.com/root4loot/urldiscover/pkg/runner"
)

func main() {
	urldiscoverOptions := options.Options{
		Concurrency: 20,
		Timeout:     10,
	}

	// initialize urldiscover and npmjack
	urldiscover := urldiscover.NewRunner(&urldiscoverOptions)
	npmjack := npmjack.NewRunner()

	// process results from urldiscover
	go func() {
		for result := range urldiscover.Results {
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

	// grab urls with urldiscover
	urldiscover.Run("hackerone.com")
}
