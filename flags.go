package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	npmjack "github.com/root4loot/npmjack/pkg/runner"
)

func (c *CLI) banner() {
	fmt.Println("\nnpmjack", version, "by", author)
}

func (c *CLI) usage() {
	w := tabwriter.NewWriter(os.Stdout, 2, 0, 3, ' ', 0)

	fmt.Fprintf(w, "Usage:\t%s [options] (-u <url> | -i <targets.txt>)\n\n", os.Args[0])

	fmt.Fprintf(w, "\nTARGETING:\n")
	fmt.Fprintf(w, "\t%s,   %s\t%s\n", "-u", "--url", "target URL")
	fmt.Fprintf(w, "\t%s,   %s\t%s\n", "-i", "--infile", "file containing targets")

	fmt.Fprintf(w, "\nCONFIGURATIONS:\n")
	fmt.Fprintf(w, "\t%s,  %s\t%s\t(Default: %d)\n", "-c", "--concurrency", "number of concurrent requests", npmjack.DefaultOptions().Concurrency)
	fmt.Fprintf(w, "\t%s,  %s\t%s\t(Default: %d %s)\n", "-t", "--timeout", "max request timeout", npmjack.DefaultOptions().Timeout, "seconds")
	fmt.Fprintf(w, "\t%s,  %s\t%s\t(Default: %d %s)\n", "-d", "--delay", "delay between requests", npmjack.DefaultOptions().Delay, "milliseconds")
	fmt.Fprintf(w, "\t%s,  %s\t%s\t(Default: %v)\n", "-r", "--resolvers", "file containing list of resolvers", "System DNS")
	fmt.Fprintf(w, "\t%s, %s\t%s\t(Default: %d %s)\n", "-dj", "--delay-jitter", "max jitter between requests", npmjack.DefaultOptions().DelayJitter, "milliseconds")
	fmt.Fprintf(w, "\t%s, %s\t%s\t(Default: %s)\n", "-ua", "--user-agent", "set user agent", npmjack.DefaultOptions().UserAgent)

	fmt.Fprintf(w, "\nOUTPUT:\n")
	fmt.Fprintf(w, "\t%s,  %s\t%s\n", "-o", "--outfile", "output results to given file")
	fmt.Fprintf(w, "\t%s, %s\t%s\n", "-hc", "--hide-claimed", "hide packages that are claimed")
	fmt.Fprintf(w, "\t%s,  %s\t%s\n", "-s", "--silence", "silence everything")
	fmt.Fprintf(w, "\t%s,  %s\t%s\n", "-v", "--verbose", "verbose output")
	fmt.Fprintf(w, "\t%s   %s\t%s\n", "  ", "--version", "display version")

	w.Flush()
	fmt.Println("")
}

// parseAndSetOptions parses the command line options and sets the options
func (c *CLI) parseFlags() {
	// TARGET
	flag.StringVar(&c.TargetURL, "url", "", "")
	flag.StringVar(&c.TargetURL, "u", "", "")
	flag.StringVar(&c.Infile, "i", "", "")
	flag.StringVar(&c.Infile, "infile", "", "")

	// CONFIGURATIONS
	flag.IntVar(&c.Concurrency, "concurrency", npmjack.DefaultOptions().Concurrency, "")
	flag.IntVar(&c.Concurrency, "c", npmjack.DefaultOptions().Concurrency, "")
	flag.IntVar(&c.Timeout, "timeout", npmjack.DefaultOptions().Timeout, "")
	flag.IntVar(&c.Timeout, "t", npmjack.DefaultOptions().Timeout, "")
	flag.IntVar(&c.Delay, "delay", npmjack.DefaultOptions().Delay, "")
	flag.IntVar(&c.Delay, "d", npmjack.DefaultOptions().Delay, "")
	flag.IntVar(&c.DelayJitter, "delay-jitter", npmjack.DefaultOptions().DelayJitter, "")
	flag.IntVar(&c.DelayJitter, "dj", npmjack.DefaultOptions().DelayJitter, "")
	flag.StringVar(&c.UserAgent, "user-agent", npmjack.DefaultOptions().UserAgent, "")
	flag.StringVar(&c.UserAgent, "ua", npmjack.DefaultOptions().UserAgent, "")
	flag.StringVar(&c.ResolversFile, "resolvers", "", "")
	flag.StringVar(&c.ResolversFile, "r", "", "")

	// OUTPUT
	flag.BoolVar(&c.Silence, "s", false, "")
	flag.BoolVar(&c.Silence, "silence", false, "")
	flag.StringVar(&c.Outfile, "o", "", "")
	flag.StringVar(&c.Outfile, "outfile", "", "")
	flag.BoolVar(&c.HideClaimed, "hc", false, "")
	flag.BoolVar(&c.HideClaimed, "hide-claimed", false, "")
	flag.BoolVar(&c.Verbose, "v", false, "")
	flag.BoolVar(&c.Verbose, "verbose", false, "")
	flag.BoolVar(&c.Help, "help", false, "")
	flag.BoolVar(&c.Help, "h", false, "")
	flag.BoolVar(&c.Version, "version", false, "")

	flag.Usage = func() {
		c.banner()
		c.usage()
	}
	flag.Parse()
}
