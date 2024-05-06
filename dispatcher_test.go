package main

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"go.uber.org/goleak"
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
		{"http://x.com", false},        // base url should fail
		{"http://x.com/", false},       // base url should fail with slash
		{"http://n.com/notok/", false}, // wrong base
		{"http://x.com/ok/", true},     // first time seen
		{"http://x.com/ok/", false},    // seen before
		{"http://x.com/ok", false},     // seen before (without slash)
		{"http://x.com/1.svg", false},  // svg
		{"http://x.com/1.png", false},  // png
		{"http://x.com/unique", true},  // unique
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

// linkMaker is a generalised way of making links
type linkMaker func() []string

// prefixer builds urls from strings to fill links
func prefixer(s ...string) linkMaker {
	x := make([]string, len(s))
	for i := range s {
		x[i] = "https://example.com/" + s[i]
	}
	return func() []string {
		return x
	}
}

// prefixerRandom makes links with a randomish url suffix
func prefixerRandom(n int) linkMaker {
	return func() []string {
		var s = make([]string, n)
		for i := range n {
			s[i] = "https://example.com/" + fmt.Sprintf("%d", rand.Int()/1e14)
		}
		return s
	}
}

func TestDispatcher(t *testing.T) {

	// HTTPTIMEOUT reset
	httpMS := 20
	HTTPTIMEOUT = (time.Millisecond * time.Duration(httpMS))
	dispatchMS := 25
	HTTPRATESEC = 100000 // effectively ignore the rate limiter
	// DISPATCHERTIMEOUT set below

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
		timeout := time.Duration(0)
		for range Dispatcher("https://example.com", []string{}, timeout) {
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
			// fails with not enough room in the buffer after about
			// 26/27 items
			workers:        20,
			linkbuffersize: 40,
			links:          prefixerRandom(3), // keep generating new links
			resultChk:      gt,                // gt means greater than
			resultNo:       27,                // more than this number expected
		},
	}

	for i, tt := range tests {
		// not parallel safe due to global use of links
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			defer goleak.VerifyNone(t)
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
			if got, want := resultNo, tt.resultNo; !tt.resultChk(resultNo, tt.resultNo) {
				t.Errorf("got %d want %d results", got, want)
			}
		})
	}
}

// TestRateLimit tests rate limits
func TestRateLimit(t *testing.T) {

	httpMS := 20
	HTTPTIMEOUT = (time.Millisecond * time.Duration(httpMS))
	dispatchMS := 25
	DISPATCHERTIMEOUT = (time.Millisecond * time.Duration(dispatchMS))
	HTTPRATESEC = 1 // reset below

	var links linkMaker
	getURLer = func(url string, searchTerms []string) (Result, []string) {
		time.Sleep(5 * time.Millisecond)
		l := links()
		return Result{
			url:     url,
			status:  200,
			matches: []SearchMatch{},
		}, l
	}

	resultCollectorTO := func(ms int, t *testing.T) int {
		timeout := time.Millisecond * time.Duration(ms)
		i := 0
		for range Dispatcher("https://example.com", []string{}, timeout) {
			i++
		}
		return i
	}

	LINKBUFFERSIZE = 200

	// note that each fake http request takes 5ms
	tests := []struct {
		links       linkMaker
		workers     int // no of GOWORKERS
		timeoutMS   int // milliseconds
		rateSec     int // num/sec
		resultAbout int
	}{
		{ // 0
			links:       prefixerRandom(2), // keep generating new links
			workers:     1,
			timeoutMS:   110,
			rateSec:     200, // 5ms per call
			resultAbout: 20,
		},
		{ // 1
			links:       prefixerRandom(2), // keep generating new links
			workers:     2,
			timeoutMS:   110,
			rateSec:     200, // 5ms per call
			resultAbout: 20,  // 20 to 21 results
		},
		{ // 2
			links:       prefixerRandom(2), // keep generating new links
			workers:     1,
			timeoutMS:   105,
			rateSec:     50, // 20ms per call
			resultAbout: 5,  //
		},
		{ // 3
			links:       prefixerRandom(2), // keep generating new links
			workers:     2,
			timeoutMS:   105,
			rateSec:     50, // 20ms per call
			resultAbout: 5,  // 5ish
		},
		{ // 4
			links:       prefixerRandom(1), // keep generating new links
			workers:     3,
			timeoutMS:   100,
			rateSec:     100, // 10ms per call
			resultAbout: 10,  // 10 to 12 results
		},
		{ // 5
			links:       prefixerRandom(2), // keep generating new links
			workers:     100,
			timeoutMS:   102,
			rateSec:     10, // 100ms per call
			resultAbout: 1,  // 1
		},
	}

	for i, tt := range tests {
		// not parallel safe due to global use of links
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			// defer goleak.VerifyNone(t)
			HTTPRATESEC = tt.rateSec
			GOWORKERS = tt.workers
			links = tt.links
			resultNo := resultCollectorTO(tt.timeoutMS, t)
			if got, want := resultNo, tt.resultAbout; got < want || got > (want+5) {
				t.Errorf("got %d want >= %d results", got, want)
			}
		})
	}
}
