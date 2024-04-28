package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestLinkError(t *testing.T) {
	err := linkError("link error")
	if got, want := err.Error(), "link error"; got != want {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestFollowURLs(t *testing.T) {

	tests := []struct {
		url string
		ok  bool
	}{
		// beware order is important
		{"http://x.com", false},  // base url should fail
		{"http://x.com/", false}, // base url should fail with slash
		{"http://x.com/ok/", true},
		{"http://x.com/ok/", false},   // seen before
		{"http://x.com/1.svg", false}, // svg
		{"http://x.com/1.png", false}, // png
		{"http://x.com/uniqe", true},  // unique
	}

	// init
	f := followURLs("http://x.com")

	for i, tt := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			if got, want := f(tt.url), tt.ok; got != want {
				t.Errorf("%s got %t want %t", tt.url, got, want)
			}
		})
	}
}

func TestDispatcher(t *testing.T) {

	// HTTPTIMEOUT reset
	httpMS := 20
	HTTPTIMEOUT = (time.Millisecond * time.Duration(httpMS))
	dispatchMS := 25
	// DISPATCHERTIMEOUT set below

	// getURLer is an indirector: see dispatcher.go
	links := []string{}
	getURLer = func(url string, searchTerms []string, done <-chan struct{}) (Result, []string) {
		time.Sleep(HTTPTIMEOUT - 200) // just less than the http timeout
		select {
		case <-done:
			fmt.Println("getURLer early done")
			return Result{}, []string{}
		default:
		}
		return Result{
			url:     "https://example.com",
			status:  200,
			matches: []SearchMatch{},
		}, links
	}

	getURLtmper = func(id int, searchTerms []string, linkSupplier <-chan string, thisResult chan<- Result,
		theseLinks chan<- []string, done <-chan struct{}) {

		go func() {
			fmt.Println("in getURLtmper", id)
			for {
				select {
				case <-done:
					fmt.Println("> got done in getURLtmper", id)
					return
				case l := <-linkSupplier:
					fmt.Println("> got link in getURLtmper", l)
					time.Sleep(HTTPTIMEOUT - 200) // just less than the http timeout
					thisResult <- Result{
						url:     "https://example.com",
						status:  200,
						matches: []SearchMatch{},
					}
					theseLinks <- links
				}
			}
		}()
	}

	resultCollector := func() int {
		i := 0
		for range Dispatcher("https://example.com", []string{}) {
			i++
		}
		return i
	}

	// build urls from strings
	prefixer := func(s ...string) []string {
		for i, x := range s {
			s[i] = "https://example.com/" + x
		}
		return s
	}

	tests := []struct {
		workers        int
		linkbuffersize int
		links          []string
		resultNo       int
		dispatchMS     int // set the dispatcher
	}{
		{
			workers:        1,
			linkbuffersize: 2,
			links:          prefixer([]string{"1", "2"}...),
			resultNo:       3, // base url + 2 links
		},
		{
			// fails with not enough room in the buffer
			workers:        1,
			linkbuffersize: 1,
			links:          prefixer([]string{"1", "2"}...),
			resultNo:       3, // base url + first two links
		},
		{ // 2
			// should proceed fine
			workers:        1,
			linkbuffersize: 1,
			links:          []string{},
			resultNo:       1, // only base url
		},
		{ // 3
			workers:        2,
			linkbuffersize: 2,
			links:          prefixer([]string{"1", "2"}...),
			resultNo:       3,
		},
		{ // 4
			workers:        2,
			linkbuffersize: 2,
			links:          prefixer([]string{"1", "2"}...),
			resultNo:       0,          // none should succeed
			dispatchMS:     httpMS - 6, // timeout before the http calls have finished
		},
		{ // 5
			workers:        2,
			linkbuffersize: 2,
			links:          prefixer([]string{"1"}...),
			resultNo:       2,
		},
		{ // 6
			workers:        1,
			linkbuffersize: 20,
			links:          prefixer(strings.Fields("a b c d e f g h i j k l m n o p")...),
			resultNo:       17, // base + 16
		},
		{ // 7
			workers:        50,
			linkbuffersize: 10,
			links:          prefixer(strings.Fields("a b c d e f")...),
			resultNo:       7, // base + 6
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			GOWORKERS = tt.workers
			LINKBUFFERSIZE = tt.linkbuffersize
			if tt.dispatchMS != 0 {
				DISPATCHERTIMEOUT = time.Millisecond * time.Duration(tt.dispatchMS)
			} else {
				// default
				DISPATCHERTIMEOUT = time.Millisecond * time.Duration(dispatchMS)
			}
			fmt.Println("HTTPTIMEOUT", HTTPTIMEOUT, "DISPATCHERTIMEOUT", DISPATCHERTIMEOUT)
			links = tt.links
			if got, want := resultCollector(), tt.resultNo; got != want {
				t.Errorf("got %d want %d results", got, want)
			}
		})
	}
}
