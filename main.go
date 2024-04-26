package main

import (
	"fmt"
	"os"

	flags "github.com/jessevdk/go-flags"
)

// Usage sets out the program usage
const Usage = `-s searchterm [-s searchterm]... <baseurl>

Look for one or more searchterms (typically constrained between double
quotes) in a website starting at <baseurl>.

Application Arguments:

 `

// Options are the command line options
type Options struct {
	SearchTerms []string `short:"s" long:"searchterm" required:"true" description:"search terms, can be specified more than once"`
	Verbose     bool     `short:"v" long:"verbose" description:"set verbose output"`
	Args        struct {
		BaseURL string `description:"base url to search"`
	} `positional-args:"yes" required:"yes"`
}

// getOptions gets the command line options
func getOptions() Options {
	var options Options
	var parser = flags.NewParser(&options, flags.Default)
	parser.Usage = Usage

	// parse args
	if _, err := parser.Parse(); err != nil {
		if !flags.WroteHelp(err) {
			parser.WriteHelp(os.Stdout)
		}
		os.Exit(1)
	}
	return options
}

// printResults prints results
func printResults(options Options, results <-chan Result) {
	pages := 0
	for r := range results {
		pages++
		switch r.err {
		case NonHTMLPageType:
			continue
		case StatusNotOk:
			fmt.Printf("%s : status %d\n", r.url, r.status)
			continue
		default:
			if r.err != nil {
				fmt.Printf("%s : error %v\n", r.url, r.err)
				continue
			}
		}

		// print url if vebose
		if options.Verbose {
			fmt.Printf("%s\n", r.url)
		}

		// matches
		if len(r.matches) > 0 {
			fmt.Printf("%s\n", r.url)
			for _, m := range r.matches {
				fmt.Printf("> match %s\n", m)
			}
		}
	}
	fmt.Println("processed", pages, "pages")
}

func main() {
	options := getOptions()
	results := Dispatcher(options.Args.BaseURL, options.SearchTerms)
	printResults(options, results)
}
