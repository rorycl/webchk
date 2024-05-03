// webchk is a command for searching all the pages on a website for one
// or more search phrases. The search phrase matches are made in
// lowercase and a naive search is made including all of the markup
// (instead of using, for example, goquery).

package main

// Thank you, Katherine Cox–Buday, for your incredibly helpful book
// "Concurrency in Go".

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
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
	LINKBUFFERSIZE = 2500
	// HTTPWORKERS is the number of concurrent web queries to run
	HTTPWORKERS = 16
	// HTTPTIMEOUT is the longest a web connection will stay open
	HTTPTIMEOUT time.Duration = 2500 * time.Millisecond
	// DISPATCHERTIMEOUT is how long the dispatcher will wait for
	// results. This is slightly longer than HTTPTIMEOUT
	DISPATCHERTIMEOUT time.Duration = 2750 * time.Millisecond

	// url suffixes to skip
	URLSuffixesToSkip = []string{".png", ".jpg", ".jpeg", ".heic", ".svg"}
	// getURLer indirects getURL for testing
	getURLer func(url string, searchTerms []string) (Result, []string) = getURL
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

	parallelURLgetter := func(inputURLs <-chan string, done <-chan struct{}) (
		<-chan Result, <-chan []string,
	) {
		results := make(chan Result)
		outputLinks := make(chan []string)

		var wg sync.WaitGroup
		wg.Add(GOWORKERS)
		for range GOWORKERS {
			go func() {
				defer wg.Done()
				for {
					select {
					case <-done:
						return
					case url := <-inputURLs:
						result, links := getURLer(url, searchTerms)
						select { // needed as getURLer may take some time
						case <-done:
							return
						default:
						}
						results <- result
						outputLinks <- links
					}
				}
			}()
		}
		go func() {
			wg.Wait()
			close(results)
			close(outputLinks)
		}()
		return results, outputLinks
	}

	links := make(chan string, LINKBUFFERSIZE)
	done := make(chan struct{})
	resultsOutput := make(chan Result)

	results, linksFound := parallelURLgetter(links, done)

	follow := followURLs(baseURL)
	links <- baseURL

	timeout := time.NewTimer(DISPATCHERTIMEOUT)
	toResetter := func() {
		if !timeout.Stop() {
			<-timeout.C
		}
		timeout.Reset(DISPATCHERTIMEOUT)
	}

	go func() {
		defer close(resultsOutput)
		for {
			select {
			case hereLinks := <-linksFound:
				toResetter() // reset timeout
				for _, l := range hereLinks {
					if !follow(l) {
						continue
					}
					select {
					case links <- l:
					default:
						fmt.Println("no space left on buffer")
						close(done)
						return
					}
				}
			case r := <-results:
				toResetter() // reset timeout
				if r.status == http.StatusTooManyRequests {
					fmt.Println("too many requests response. quitting...")
					close(done)
					return
				}
				resultsOutput <- r
			case <-timeout.C:
				close(done)
				return
			}
		}
	}()
	return resultsOutput
}
