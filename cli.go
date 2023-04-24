package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/gologger/levels"
	"github.com/root4loot/npmjack/pkg/log"
	npmjack "github.com/root4loot/npmjack/pkg/runner"
)

type CLI struct {
	TargetURL             string // target URL
	Concurrency           int    // number of concurrent requests
	Timeout               int    // Request timeout duration (in seconds)
	Delay                 int    // delay between each request (in ms)
	DelayJitter           int    // maximum jitter to add to delay (in ms)
	ResponseHeaderTimeout int    // Response header timeout duration (in seconds)
	UserAgent             string // custom user-agent
	Infile                string // file containin targets (newline separated)
	Outfile               string // file to write results
	HideClaimed           bool   // hide claimed packages
	Verbose               bool   // hide info
	Silence               bool   // suppress output from console
	Version               bool   // print version
	Writer                *tabwriter.Writer
	Help                  bool // print help
}

const author = "@danielantonsen"

func main() {
	var targets []string
	var err error
	cli := newCLI()
	cli.initialize()
	npmjack := npmjack.NewRunner()

	npmjack.Options.Concurrency = cli.Concurrency
	npmjack.Options.Timeout = cli.Timeout
	npmjack.Options.Delay = cli.Delay
	npmjack.Options.DelayJitter = cli.DelayJitter
	npmjack.Options.UserAgent = cli.UserAgent
	npmjack.Options.Verbose = cli.Verbose

	cli.Writer = tabwriter.NewWriter(os.Stdout, 27, 0, 0, ' ', tabwriter.TabIndent)
	if !cli.Silence && !cli.Verbose {
		fmt.Println("")
		fmt.Fprintln(cli.Writer, "\tPACKAGE\tNAMESPACE            CLAIMED   SOURCE\t")
		fmt.Fprintln(cli.Writer, "\t-------\t---------            -------   ------\t")
	}

	if cli.hasStdin() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			url := scanner.Text()
			cli.processResults(npmjack)
			npmjack.Run(url)
		}
	} else if cli.hasInfile() {
		if targets, err = cli.readFileLines(); err != nil {
			log.Fatalf("Error reading file: ", err)
		}
	} else if cli.hasTarget() {
		targets = cli.getTargets()
	}

	for _, target := range targets {
		cli.processResults(npmjack)
		npmjack.Run(target)
	}
}

// processResults is a goroutine that processes the results as they come in
func (c *CLI) processResults(runner *npmjack.Runner) {
	go func() {
		for result := range runner.Results {
			if !c.Silence {
				if result.Packages != nil {
					for _, pkg := range result.Packages {
						if pkg.Namespace == " " {
							pkg.Namespace = strings.Repeat(" ", 15) // create a string with 15 blank spaces
						}

						if pkg.Claimed {
							if !c.HideClaimed {
								fmt.Fprintf(c.Writer, "%s\t%-12s         %-12s%s\n", pkg.Name, pkg.Namespace, "No", result.RequestURL)
							}
						} else {
							fmt.Fprintf(c.Writer, "%s\t%-12s         %-12s%s\n", pkg.Name, pkg.Namespace, "Yes", result.RequestURL)
						}
					}
				}
			}
			if c.hasOutfile() {
				c.writeToFile([]string{strconv.Itoa(result.StatusCode) + " " + result.RequestURL})
			}
		}
	}()

	c.Writer.Flush()
}

func (c *CLI) initialize() {
	c.parseFlags()
	c.checkForExits()

	if c.Silence {
		gologger.DefaultLogger.SetMaxLevel(levels.LevelSilent)
	} else if c.Verbose {
		gologger.DefaultLogger.SetMaxLevel(levels.LevelDebug)
	}
}

func newCLI() *CLI {
	return &CLI{}
}

// writeToFile writes the given lines to the given file
func (c *CLI) writeToFile(lines []string) {
	file, err := os.OpenFile(c.Outfile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		log.Errorf("could not open file: %v", err)
	}
	defer file.Close()

	for i := range lines {
		if _, err := file.WriteString(lines[i] + "\n"); err != nil {
			log.Errorf("could not write line to file: %v", err)
		}
	}
}

// checkForExits checks for the presence of the -h|--help and -v|--version flags
func (c *CLI) checkForExits() {
	if c.Help {
		c.banner()
		c.usage()
		os.Exit(0)
	}
	if c.Version {
		fmt.Println("npmjack ", version)
		os.Exit(0)
	}

	if !c.hasStdin() && !c.hasInfile() && !c.hasTarget() {
		log.Fatalf("%s", "Missing target")
	}
}

// getTargets returns the targets to be used for the scan
func (c *CLI) getTargets() (targets []string) {
	if c.hasTarget() {
		if strings.Contains(c.TargetURL, ",") {
			c.TargetURL = strings.ReplaceAll(c.TargetURL, " ", "")
			targets = strings.Split(c.TargetURL, ",")
		} else {
			targets = append(targets, c.TargetURL)
		}
	}
	return
}

// ReadFileLines reads a file line by line
func (c *CLI) readFileLines() (lines []string, err error) {
	file, err := os.Open(c.Infile)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return
}

// hasStdin determines if the user has piped input
func (c *CLI) hasStdin() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	mode := stat.Mode()

	isPipedFromChrDev := (mode & os.ModeCharDevice) == 0
	isPipedFromFIFO := (mode & os.ModeNamedPipe) != 0

	return isPipedFromChrDev || isPipedFromFIFO
}

// hasTarget determines if the user has provided a target
func (c *CLI) hasTarget() bool {
	return c.TargetURL != ""
}

// hasInfile determines if the user has provided an input file
func (c *CLI) hasInfile() bool {
	return c.Infile != ""
}

// hasOutfile determines if the user has provided an output file
func (c *CLI) hasOutfile() bool {
	return c.Outfile != ""
}
