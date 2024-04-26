// webchk is a command for searching all the pages on a website for one
// or more search phrases. The search phrase matches are made in
// lowercase and a naive search is made including all of the markup
// (instead of using, for example, goquery).
package main

import (
	"fmt"
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
	// HTTPWORKERS is the number of concurrent web queries to run
	HTTPWORKERS = 16
	// HTTPTIMEOUT is the longest a web connection will stay open
	HTTPTIMEOUT time.Duration = 3 * time.Second
	// Sentinel error for non html pages
	NonHTMLPageType = linkError("NonHTMLPageType")
	StatusNotOk     = linkError("StatusNotOk")
)

var (
	// GOWORKERS is the number of worker goroutines to start
	GOWORKERS = 8
	// LINKBUFFERSIZE is the size of the link buffer during processing
	LINKBUFFERSIZE = 1000
	// url suffixes to skip
	URLSuffixesToSkip = []string{".png", ".jpg", ".jpeg", ".heic", ".svg"}
	// getURLer indirects getURL for testing
	getURLer func(url string, searchTerms []string) (Result, []string) = getURL
)

// followURLs is a closure which returns true if a url has not been seen
// before and the provided url matches the baseURL and does not match
// one of the provided URLSuffixes
func followURLs(baseURL string) func(u string) bool {
	var uniqueURLs sync.Map
	uniqueURLs.Store(baseURL, true)
	return func(u string) bool {
		u = strings.TrimSuffix(u, "/")
		if !strings.Contains(u, baseURL) {
			return false
		}
		if _, ok := uniqueURLs.Load(u); ok {
			return false
		}
		for _, skip := range URLSuffixesToSkip {
			if strings.HasSuffix(u, skip) {
				return false
			}
		}
		uniqueURLs.Store(u, true)
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

	results := make(chan Result)
	follow := followURLs(baseURL) // determines if links have already been seen

	// links is a buffer for storing recursively processed urls
	links := make(chan string, LINKBUFFERSIZE)
	links <- baseURL

	initialised := false
	var once sync.Once

	var wg sync.WaitGroup
	for i := 0; i < GOWORKERS; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case url, ok := <-links:
					if !ok {
						return // the channel is closed
					}
					once.Do(func() { initialised = true })
					thisResult, theseLinks := getURLer(url, searchTerms)
					results <- thisResult
					for _, l := range theseLinks {
						if follow(l) {
							select {
							case links <- l:
							default:
								// there is no buffer space in links; abort
								fmt.Printf("no space in links channel to add item %s", url)
								fmt.Println("...aborting")
								return
							}
						}
					}
				default: // no more urls to read
					if initialised {
						return
					}
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(links)
		close(results)
	}()
	return results
}
