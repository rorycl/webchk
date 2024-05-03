package main

import (
	"fmt"
	"math/rand"
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
	type linkMaker func() []string

	// build urls from strings to fill links
	prefixer := func(s ...string) linkMaker {
		x := make([]string, len(s))
		for i := range s {
			x[i] = "https://example.com/" + s[i]
		}
		return func() []string {
			return x
		}
	}

	// fixed random number source to fill links
	prefixerRandom := func(n int) linkMaker {
		var rns = rand.New(rand.NewSource(1))
		return func() []string {
			var s = make([]string, n)
			for i := range n {
				s[i] = "https://example.com/" + fmt.Sprintf("%d", rns.Int()/1e14)
			}
			return s
		}
	}

	var links linkMaker
	getURLer = func(url string, searchTerms []string) (Result, []string) {
		time.Sleep(HTTPTIMEOUT - 200) // just less than the http timeout
		l := links()
		return Result{
			url:     url,
			status:  200,
			matches: []SearchMatch{},
		}, l
	}

	resultCollector := func() int {
		i := 0
		for range Dispatcher("https://example.com", []string{}) {
			i++
		}
		return i
	}

	type resultChecker func(i, j int) bool
	eq := func(i, j int) bool {
		return i == j
	}
	gt := func(i, j int) bool {
		return i > j
	}
	/*
		gte := func(i, j int) bool {
			return i >= j
		}
	*/

	tests := []struct {
		workers        int
		linkbuffersize int
		links          linkMaker
		resultChk      resultChecker
		resultNo       int
		dispatchMS     int // set the dispatcher
	}{
		{
			workers:        1,
			linkbuffersize: 2,
			links:          prefixer([]string{"1", "2"}...),
			resultChk:      eq, // equality checker
			resultNo:       3,  // there will be 3 results
		},
		{ // 1
			// fails with not enough room in the buffer
			workers:        1,
			linkbuffersize: 1,
			links:          prefixer([]string{"1", "2"}...),
			resultChk:      gt,
			resultNo:       0,
		},
		{ // 2
			// should proceed fine
			workers:        1,
			linkbuffersize: 1,
			links:          prefixer([]string{}...),
			resultChk:      eq,
			resultNo:       1,
		},
		{ // 3
			workers:        2,
			linkbuffersize: 2,
			links:          prefixer([]string{"1", "2"}...),
			resultChk:      eq,
			resultNo:       3,
		},
		{ // 4
			workers:        2,
			linkbuffersize: 2,
			links:          prefixer([]string{"1", "2"}...),
			resultChk:      eq,
			resultNo:       0,
			dispatchMS:     httpMS - 6, // timeout before the http calls have finished
		},
		{ // 5
			workers:        2,
			linkbuffersize: 2,
			links:          prefixer([]string{"1"}...),
			resultChk:      eq,
			resultNo:       2,
		},
		{ // 6
			workers:        5,
			linkbuffersize: 20,
			links:          prefixer(strings.Fields("a b c d e f g h i j k l m n o p")...),
			resultChk:      eq,
			resultNo:       17,
		},
		{ // 7
			workers:        50,
			linkbuffersize: 10,
			links:          prefixer(strings.Fields("a b c d e f")...),
			resultChk:      eq,
			resultNo:       7,
		},
		{ // 8
			// fails with not enough room in the buffer after about 33
			// items
			workers:        20,
			linkbuffersize: 40,
			links:          prefixerRandom(3), // keep generating new links
			resultChk:      gt,                // gt means greater than
			resultNo:       28,                // more than this number expected
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			// not parallel safe due to global use of links
			GOWORKERS = tt.workers
			LINKBUFFERSIZE = tt.linkbuffersize
			if tt.dispatchMS != 0 {
				DISPATCHERTIMEOUT = time.Millisecond * time.Duration(tt.dispatchMS)
			} else {
				// default
				DISPATCHERTIMEOUT = time.Millisecond * time.Duration(dispatchMS)
			}
			links = tt.links
			resultNo := resultCollector()
			// t.Log(resultNo)
			if got, want := resultNo, tt.resultNo; !tt.resultChk(resultNo, tt.resultNo) {
				t.Errorf("got %d want %d results", got, want)
			}
		})
	}
}
