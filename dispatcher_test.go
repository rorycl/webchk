package main

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
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

func TestNewDispatch(t *testing.T) {

	tp := func(td string) time.Duration {
		d, err := time.ParseDuration(td)
		if err != nil {
			t.Fatalf("time parsing error: %v", err)
		}
		return d
	}

	tests := []struct {
		name               string
		baseURL            string
		workers            int
		linkBufferSize     int
		httpRateSec        int
		searchTerms        []string
		dispatcherTimeout  time.Duration
		timeout            time.Duration
		client             *getClient
		wantWorkers        int
		wantLinkBufferSize int
		wantHttpRateSec    int
	}{
		{
			name:               "check_defaults",
			baseURL:            "https://example.com",
			workers:            0,
			linkBufferSize:     0,
			httpRateSec:        0,
			searchTerms:        []string{"hi"},
			dispatcherTimeout:  DISPATCHERTIMEOUT,
			timeout:            tp("2m"),
			client:             &getClient{},
			wantWorkers:        HTTPWORKERS,
			wantLinkBufferSize: LINKBUFFERSIZE,
			wantHttpRateSec:    HTTPRATESEC,
		},
		{
			name:               "check_custom",
			baseURL:            "https://example.com",
			workers:            4,
			linkBufferSize:     20_000,
			httpRateSec:        195,
			searchTerms:        []string{"hi", "there"},
			dispatcherTimeout:  DISPATCHERTIMEOUT,
			timeout:            tp("2m15s"),
			client:             &getClient{},
			wantWorkers:        4,
			wantLinkBufferSize: 20_000,
			wantHttpRateSec:    195,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDispatch(
				tt.baseURL,
				tt.workers,
				tt.linkBufferSize,
				tt.httpRateSec,
				tt.searchTerms,
				tt.dispatcherTimeout,
				tt.timeout,
				tt.client,
			)
			if got, want := d.workers, tt.wantWorkers; got != want {
				t.Errorf("workers got %v != want %v", got, want)
			}
			if got, want := d.linkBufferSize, tt.wantLinkBufferSize; got != want {
				t.Errorf("buffersize got %v != want %v", got, want)
			}
			if got, want := d.httpRateSec, tt.wantHttpRateSec; got != want {
				t.Errorf("ratesec got %v != want %v", got, want)
			}
			if diff := cmp.Diff(d.searchTerms, tt.searchTerms); diff != "" {
				t.Errorf("searchterms diff %v", diff)
			}
			if got, want := d.dispatcherTimeout, tt.dispatcherTimeout; got != want {
				t.Errorf("dispatcherTimeout got %v != want %v", got, want)
			}
			if got, want := d.ctxTimeout, tt.timeout; got != want {
				t.Errorf("global timeout got %v != want %v", got, want)
			}
			// tt.client not of interest
		})
	}
}

func TestDispatcher(t *testing.T) {

	httpMS := 20
	httpTimeout := (time.Millisecond * time.Duration(httpMS))
	dispatchMS := 25
	httpRateSec := 100000 // effectively ignore the rate limiter
	invocationTimeout := (time.Second * 2)

	var links linkMaker
	getURLer := func(url, referrer string, searchTerms []string) (Result, []string) {
		time.Sleep(httpTimeout - 200) // just less than the http timeout
		l := links()
		return Result{
			url:     url,
			status:  200,
			matches: []SearchMatch{},
		}, l
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
		dispatchMS     int // set the dispatcher timeout if not thistest.dispatchMS
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
			var timeout time.Duration
			if tt.dispatchMS != 0 {
				timeout = time.Millisecond * time.Duration(tt.dispatchMS)
			} else { // default
				timeout = time.Millisecond * time.Duration(dispatchMS)
			}
			links = tt.links

			gc := NewGetClient(tt.workers, httpTimeout)
			gc.getURL = getURLer

			d := NewDispatch("https://example.com",
				tt.workers,
				tt.linkbuffersize,
				httpRateSec,
				[]string{},
				timeout,
				invocationTimeout,
				gc,
			)
			resultNo := 0
			for range d.Dispatcher() {
				resultNo++
			}
			if got, want := resultNo, tt.resultNo; !tt.resultChk(resultNo, tt.resultNo) {
				t.Errorf("got %d want %d results", got, want)
			}
		})
	}
}

// TestRateLimit tests rate limits
func TestRateLimit(t *testing.T) {

	var links linkMaker
	getURLer := func(url, referrer string, searchTerms []string) (Result, []string) {
		time.Sleep(5 * time.Millisecond)
		l := links()
		return Result{
			url:     url,
			status:  200,
			matches: []SearchMatch{},
		}, l
	}

	// note that each fake http request takes 5ms
	tests := []struct {
		links           linkMaker
		workers         int // no of GOWORKERS
		invokeTimeoutMS int // milliseconds
		rateSec         int // num/sec
		resultAbout     int
	}{
		{ // 0
			links:           prefixerRandom(2), // keep generating new links
			workers:         1,
			invokeTimeoutMS: 110,
			rateSec:         200, // 5ms per call
			resultAbout:     20,
		},
		{ // 1
			links:           prefixerRandom(2), // keep generating new links
			workers:         2,
			invokeTimeoutMS: 110,
			rateSec:         200, // 5ms per call
			resultAbout:     20,  // 20 to 21 results
		},
		{ // 2
			links:           prefixerRandom(2), // keep generating new links
			workers:         1,
			invokeTimeoutMS: 105,
			rateSec:         50, // 20ms per call
			resultAbout:     5,  //
		},
		{ // 3
			links:           prefixerRandom(2), // keep generating new links
			workers:         2,
			invokeTimeoutMS: 105,
			rateSec:         50, // 20ms per call
			resultAbout:     5,  // 5ish
		},
		{ // 4
			links:           prefixerRandom(1), // keep generating new links
			workers:         3,
			invokeTimeoutMS: 100,
			rateSec:         100, // 10ms per call
			resultAbout:     10,  // 10 to 12 results
		},
		{ // 5
			links:           prefixerRandom(2), // keep generating new links
			workers:         100,
			invokeTimeoutMS: 102,
			rateSec:         10, // 100ms per call
			resultAbout:     1,  // 1
		},
	}

	for i, tt := range tests {
		// not parallel safe due to global use of links
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			defer goleak.VerifyNone(t)

			httpMS := 20
			httpTimeout := (time.Millisecond * time.Duration(httpMS))
			linkBufferSize := 200
			dispatcherTimeout := httpTimeout * 2

			links = tt.links

			gc := NewGetClient(HTTPWORKERS, httpTimeout)
			gc.getURL = getURLer

			d := NewDispatch("https://example.com",
				tt.workers,
				linkBufferSize,
				tt.rateSec,
				[]string{},
				dispatcherTimeout,
				time.Millisecond*time.Duration(tt.invokeTimeoutMS),
				gc,
			)
			resultNo := 0
			for range d.Dispatcher() {
				resultNo++
			}

			// t.Logf("got %d sort of want %d", resultNo, tt.resultAbout)
			if got, want := resultNo, tt.resultAbout; got < want || got > (want+5) {
				t.Errorf("got %d want >= %d results", got, want)
			}
		})
	}
}
