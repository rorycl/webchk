// webchk is a command for searching all the pages on a website for one
// or more search phrases. The search phrase matches are made in
// lowercase and a naive search is made including all of the markup
// (instead of using, for example, goquery).
package main

import (
	"errors"
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

	// linkFound is a found like to be inserted into the links buffered
	// channel
	linkFound := make(chan string)

	done := make(chan struct{})

	// linkManager manages a channel with a buffer of links to process,
	// limited to LINKBUFFERSIZE
	linkManager := func(linkFound <-chan string) <-chan string {

		links := make(chan string, LINKBUFFERSIZE)
		links <- baseURL

		follow := followURLs(baseURL) // whether to follow urls

		timeout := time.NewTimer(DISPATCHERTIMEOUT)
		toResetter := func() {
			if !timeout.Stop() {
				<-timeout.C
			}
			timeout.Reset(DISPATCHERTIMEOUT)
		}

		var wg sync.WaitGroup
		go func() {
			wg.Add(1)
			defer close(links)
			defer wg.Done()
			for {
				select {
				case l := <-linkFound:
					toResetter() // reset timeout
					if !follow(l) {
						continue
					}
					select { // select here in case no space left in links
					case links <- l:
					default:
						done <- struct{}{}
						return
					}
				case <-timeout.C:
					fmt.Println("manager timing out")
					done <- struct{}{}
					return
				}
			}
		}()
		go func() { // keep the above goroutine running
			wg.Wait()
			return
		}()
		return links
	}

	// linkConsumerMaker consumes links from linkManager while making
	// more links
	linkConsumerMaker := func(links <-chan string, linkFound chan<- string) <-chan Result {

		results := make(chan Result)

		var wg sync.WaitGroup
		wg.Add(GOWORKERS)
		for i := range GOWORKERS {
			go func() {
				ii := i
				fmt.Println("go routine", ii, "started")
				defer wg.Done()
				defer fmt.Println("go routine", ii, "done")
				for {
					select {
					case <-done:
						fmt.Println("top done", ii)
						return
					case url, ok := <-links:
						if !ok { // closed channel
							fmt.Println("links done", ii)
							return
						}
						thisResult, theseLinks := getURLer(url, searchTerms, done)
						// because getURLer takes a while, check if done
						// has been completed
						select {
						case <-done:
							fmt.Println("post result return b", ii)
							// links = nil
							return
						default:
						}
						results <- thisResult
						for _, u := range theseLinks {
							linkFound <- u
						}
					}
				}
			}()
		}

		go func() { // keep the above goroutine running
			wg.Wait()
			close(results)
		}()
		return results
	}

	links := linkManager(linkFound)
	return linkConsumerMaker(links, linkFound)
}
