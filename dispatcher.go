// webchk is a command for searching all the pages on a website for one
// or more search phrases. The search phrase matches are made in
// lowercase and a naive search is made including all of the markup
// (instead of using, for example, goquery).

package main

// Thank you, Katherine Cox–Buday, for your incredibly helpful book
// "Concurrency in Go".

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
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
	// GOWORKERS is the number of worker goroutines to start processing
	// http queries
	GOWORKERS = 8
	// LINKBUFFERSIZE is the size of the link buffer during processing
	LINKBUFFERSIZE = 2500
	// HTTPWORKERS is the number of concurrent web queries to run; this
	// doesn't make sense to make much less than GOWORKERS
	HTTPWORKERS = 8
	// HTTPRATESEC is the rate of http requests to process per second
	// across all GOWORKERS
	HTTPRATESEC = 10
	// HTTPTIMEOUT is the longest a web connection will stay open
	HTTPTIMEOUT time.Duration = 1750 * time.Millisecond
	// DISPATCHERTIMEOUT is how long the dispatcher will wait for
	// results. This is slightly longer than HTTPTIMEOUT
	DISPATCHERTIMEOUT time.Duration = 1800 * time.Millisecond

	// url suffixes to skip
	URLSuffixesToSkip = []string{".png", ".jpg", ".jpeg", ".heic", ".svg"}
	// getURLer indirects getURL for testing
	getURLer func(url, referrer string, searchTerms []string) (Result, []string) = getURL
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
// one of the provided URLSuffixes. followURLs should only used in a
// fully contained manner (by a single func) and therefore does not need
// to be protected by a synchronisation primitive such as sync.Map.
func followURLs(baseURL string) func(u string) bool {
	uniqueURLs := map[string]bool{}
	uniqueURLs[baseURL] = true
	return func(u string) bool {
		u = strings.TrimSuffix(u, "/") // shouldn't be necessary
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
// produce more links than can be easily processed, a buffered channel
// is used to store urls waiting to be processed. If the channel becomes
// full the program will start to shut down.
func Dispatcher(baseURL string, searchTerms []string, ctxTimeout time.Duration) <-chan Result {

	if DISPATCHERTIMEOUT < HTTPTIMEOUT {
		fmt.Println(ErrDispatchTimeoutTooSmall)
	}

	type refLink struct {
		url, referrer string
	}

	concurrentURLgetter := func(ctx context.Context, inputURLs <-chan refLink) (
		<-chan Result, <-chan []refLink,
	) {
		results := make(chan Result)
		outputLinks := make(chan []refLink)

		// use the x/time/rate token bucket rate limiter
		rateLimit := rate.NewLimiter(rate.Limit(HTTPRATESEC), 1)

		var wg sync.WaitGroup
		wg.Add(GOWORKERS)
		for range GOWORKERS {
			go func() {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					case rl := <-inputURLs:
						err := rateLimit.Wait(ctx)
						if err != nil {
							return // ctx timeout
						}
						result, links := getURLer(rl.url, rl.referrer, searchTerms)
						// done checks for each send of the results from
						// getURLer are needed as getURLer may take some
						// time. The guards are to stop sends causing
						// goroutine leaks.
						select {
						case <-ctx.Done():
							return
						case results <- result:
						}
						refLinks := []refLink{}
						for _, l := range links {
							refLinks = append(refLinks, refLink{l, result.url})
						}
						select {
						case <-ctx.Done():
							return
						case outputLinks <- refLinks:
						}
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

	links := make(chan refLink, LINKBUFFERSIZE)
	resultsOutput := make(chan Result)

	var ctx context.Context
	var cancel context.CancelFunc
	switch {
	case ctxTimeout <= 0:
		ctx, cancel = context.WithCancel(context.Background())
	default:
		ctx, cancel = context.WithTimeout(context.Background(), ctxTimeout)
	}

	results, linksFound := concurrentURLgetter(ctx, links)

	follow := followURLs(baseURL)
	links <- refLink{url: baseURL, referrer: "/"} // start links with baseurl

	// define timeout and timeout reset function
	timeout := time.NewTimer(DISPATCHERTIMEOUT)
	toResetter := func() {
		if !timeout.Stop() {
			<-timeout.C
		}
		timeout.Reset(DISPATCHERTIMEOUT)
	}

	// this func is the main coordinator of Dispatcher, putting incoming
	// links from concurrentURLgetter onto the links buffered channel if
	// they have not already been seen by follow() and sending results
	// to the resultsOutput channel for consumption by the user.
	go func() {
		defer close(resultsOutput)
		defer close(links)
		defer func() {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				fmt.Printf("deadline of %s exceeded. quitting...\n", ctxTimeout)
			}
			cancel()
		}()
		for {
			select {
			case hereLinks, ok := <-linksFound:
				if !ok {
					return
				}
				for _, l := range hereLinks {
					if !follow(l.url) {
						continue
					}
					select {
					case links <- l:
					default:
						fmt.Println("no space left on buffer")
						return
					}
				}
			case r, ok := <-results:
				if !ok {
					return
				}
				toResetter() // reset timeout
				if r.status == http.StatusTooManyRequests {
					fmt.Println("too many requests error. quitting...")
					return
				}
				resultsOutput <- r
			case <-timeout.C:
				return
			}
		}
	}()
	return resultsOutput
}
