package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	flags "github.com/jessevdk/go-flags"
)

// Usage sets out the program usage
const Usage = `-s searchterm [-s searchterm]... <baseurl>

Look for one or more searchterms (typically constrained between double
quotes) in a website starting at <baseurl>.

Application Arguments:

 `

// errorForOSExit signals that an os.Exit(1) is required
var errorForOSExit = errors.New("osexit")

// Options are the command line options
type Options struct {
	SearchTerms []string `short:"s" long:"searchterm" required:"true" description:"search terms, can be specified more than once"`
	Verbose     bool     `short:"v" long:"verbose" description:"set verbose output"`
	BufferSize  int      `short:"z" long:"buffersize" description:"size of links buffer" default:"2500"`
	Workers     int      `short:"w" long:"workers" description:"number of goroutine workers" default:"8"`
	HTTPWorkers int      `short:"x" long:"httpworkers" description:"number of http workers" default:"16"`
	Args        struct {
		BaseURL string `description:"base url to search"`
	} `positional-args:"yes" required:"yes"`
}

// getOptions gets the command line options
func getOptions() (Options, error) {
	var options Options
	var parser = flags.NewParser(&options, flags.Default)
	parser.Usage = Usage

	// parse args
	if _, err := parser.Parse(); err != nil {
		if !flags.WroteHelp(err) {
			parser.WriteHelp(os.Stdout)
		}
		return options, errorForOSExit
	}
	if options.BufferSize > 0 && options.BufferSize != LINKBUFFERSIZE {
		LINKBUFFERSIZE = options.BufferSize
	}
	if options.Workers > 0 && options.Workers != GOWORKERS {
		GOWORKERS = options.Workers
	}
	if options.HTTPWorkers > 0 && options.HTTPWorkers != HTTPWORKERS {
		HTTPWORKERS = options.HTTPWorkers
	}
	return options, nil
}

// output sets the io.Writer for output
var output io.Writer = os.Stdout

// printResults prints results
func printResults(options Options, results <-chan Result) {

	fmt.Fprintf(output, "\nCommencing search of %s:\n", options.Args.BaseURL)

	pages := 0
	for r := range results {
		pages++
		switch r.err {
		case NonHTMLPageType:
			continue
		case StatusNotOk:
			fmt.Fprintf(output, "%s : status %d\n", r.url, r.status)
			continue
		default:
			if r.err != nil {
				fmt.Fprintf(output, "%s : error %v\n", r.url, r.err)
				continue
			}
		}

		// print url if vebose
		if options.Verbose {
			fmt.Fprintf(output, "%s\n", r.url)
		}

		// matches
		if len(r.matches) > 0 {
			fmt.Fprintf(output, "%s\n", r.url)
			for _, m := range r.matches {
				fmt.Fprintf(output, "> %s\n", m)
			}
		}
	}
	fmt.Fprintln(output, "processed", pages, "pages")
}

func main() {
	options, err := getOptions()
	if errors.Is(errorForOSExit, err) {
		os.Exit(1)
	}
	results := Dispatcher(options.Args.BaseURL, options.SearchTerms)
	printResults(options, results)
}
