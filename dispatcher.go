// webchk is a command for searching all the pages on a website for one
// or more search phrases. The search phrase matches are made in
// lowercase and a naive search is made including all of the markup
// (instead of using, for example, goquery).
package main

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// linkError is a type for sentinel errors
type linkError string

// Error meets the error interface requirement
func (err linkError) Error() string {
	return string(err)
}

const (
	// Sentinel error for non html pages
	NonHTMLPageType = linkError("NonHTMLPageType")
	StatusNotOk     = linkError("StatusNotOk")
)

var (
	// GOWORKERS is the number of worker goroutines to start
	GOWORKERS = 8
	// LINKBUFFERSIZE is the size of the link buffer during processing
	LINKBUFFERSIZE = 1000
	// HTTPWORKERS is the number of concurrent web queries to run
	HTTPWORKERS = 16
	// HTTPTIMEOUT is the longest a web connection will stay open
	HTTPTIMEOUT time.Duration = 1750 * time.Millisecond
	// DISPATCHERTIMEOUT is how long the dispatcher will wait for
	// results. This is slightly longer than HTTPTIMEOUT
	DISPATCHERTIMEOUT time.Duration = 2000 * time.Millisecond

	// url suffixes to skip
	URLSuffixesToSkip = []string{".png", ".jpg", ".jpeg", ".heic", ".svg"}
	// getURLer indirects getURL for testing
	getURLer func(url string, searchTerms []string, done <-chan struct{}) (Result, []string) = getURL

	getURLtmper func(id int, searchTerms []string, links <-chan string, thisResult chan<- Result,
		theseLinks chan<- []string, done <-chan struct{}) = getURLtmp
)

var (
	// ErrDispatchTimeoutTooSmall is an error message when the
	// DISPATCHERTIMEOUT is set too small
	ErrDispatchTimeoutTooSmall = errors.New(`
	dispatcher timeout should not be smaller than the httptimeout as the
	dispatcher will stop processing before the web calls have been
	terminated.
	`)
)

// followURLs is a closure which returns true if a url has not been seen
// before and the provided url matches the baseURL and does not match
// one of the provided URLSuffixes. Due to containment, this does not
// use sync.Map
func followURLs(baseURL string) func(u string) bool {
	uniqueURLs := map[string]bool{}
	uniqueURLs[baseURL] = true
	return func(u string) bool {
		u = strings.TrimSuffix(u, "/")
		if !strings.Contains(u, baseURL) {
			return false
		}
		if _, ok := uniqueURLs[u]; ok {
			return false
		}
		for _, skip := range URLSuffixesToSkip {
			if strings.HasSuffix(u, skip) {
				return false
			}
		}
		uniqueURLs[u] = true
		return true
	}
}

// Dispatcher is a function for launching worker goroutines to process
// getURL functions to produce Results. Since the initial page(s)
// produce more links than can be easily processed, a buffered channel is
// used to store urls waiting to be processed. If the channel becomes
// full additional urls are dropped and the program will start to shut
// down.
func Dispatcher(baseURL string, searchTerms []string) <-chan Result {

	if DISPATCHERTIMEOUT < HTTPTIMEOUT {
		fmt.Println(ErrDispatchTimeoutTooSmall)
	}

	done := make(chan struct{})

	// linkManager manages a channel with a buffer of links to process,
	// limited to LINKBUFFERSIZE
	linkManager := func() (<-chan string, chan<- string) {

		// links is a buffer of urls to process
		links := make(chan string, LINKBUFFERSIZE)
		// linkFound is a link to be inserted into links
		linkFound := make(chan string)

		links <- baseURL

		follow := followURLs(baseURL) // whether to follow urls

		timeout := time.NewTimer(DISPATCHERTIMEOUT)
		toResetter := func() {
			if !timeout.Stop() {
				<-timeout.C
			}
			timeout.Reset(DISPATCHERTIMEOUT)
		}
		go func() {
			for {
				select {
				case l := <-linkFound:
					fmt.Println("manager resetting with", l)
					toResetter() // reset timeout
					if !follow(l) {
						fmt.Println("   follow was false for", l)
						continue
					}
					select { // select here in case no space left in links
					case links <- l:
					default:
						fmt.Println("no space left on buffer")
						close(links)
						close(done)
						return
					}
				case <-timeout.C:
					fmt.Println("manager timing out")
					fmt.Printf("cap %d len %d\n", cap(links), len(links))
					close(links)
					close(done)
					return
				}
			}
			fmt.Println("manager exited")
		}()
		return links, linkFound
	}

	linkConsumer := func(links <-chan string) (<-chan Result, <-chan []string) {

		getResult := make(chan Result)
		getLinkResults := make(chan []string)

		for i := range GOWORKERS {
			ii := i
			go getURLtmper(ii, searchTerms, links, getResult, getLinkResults, done)
		}
		return getResult, getLinkResults
	}

	// resulter consumes links from linkManager while receiving more
	// links from linkConsumer which are sent back to linkManager
	linkReturner := func(getResult <-chan Result, getLinkResults <-chan []string, linkFound chan<- string) <-chan Result {
		finalResults := make(chan Result)
		go func() {
			for {
				select {
				case <-done:
					close(finalResults)
					return
				case lr := <-getResult:
					fmt.Println("   sending results")
					finalResults <- lr
				case hLinks, ok := <-getLinkResults:
					if !ok {
						close(finalResults)
						return
					}
					fmt.Println("links received", hLinks)
					fmt.Println("   sending links")
					for _, l := range hLinks {
						linkFound <- l
					}
				}
			}
		}()
		return finalResults
	}

	links, linkFound := linkManager()
	localResults, localLinks := linkConsumer(links)
	return linkReturner(localResults, localLinks, linkFound)
}
