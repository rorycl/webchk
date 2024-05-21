package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestGetOptions(t *testing.T) {

	tests := []struct {
		argString   string
		SearchTerms []string
		Verbose     bool
		BaseURL     string
		BufferSize  int
		QuerySec    int
		Timeout     string // valid time.ParseDuration string needed
		HTTPWorkers int
		Workers     int
		ok          bool
	}{
		{ // 0
			argString: "<prog> -h",
			ok:        false, // actually osexits
		},
		{ // 1
			argString: `<prog> https://www.test.com`,
			ok:        false,
		},
		{ // 2
			argString:   `<prog> -s "hi" https://www.test.com`,
			SearchTerms: []string{"hi"},
			Verbose:     false,
			BaseURL:     "https://www.test.com",
			ok:          true,
		},
		{ // 3
			argString:   `<prog> -s "hi" -s "there" https://www.test.com`,
			SearchTerms: []string{"hi", "there"},
			Verbose:     false,
			BaseURL:     "https://www.test.com",
			ok:          true,
		},
		{ // 4
			argString:   `<prog> -v -s "hi" -s "there" https://www.test.com`,
			SearchTerms: []string{"hi", "there"},
			Verbose:     true,
			BaseURL:     "https://www.test.com",
			ok:          true,
		},
		{ // 5
			// no base url
			argString: `<prog> -v -s "hi" -s "there"`,
			ok:        false,
		},
		{ // 6
			// unknown flag
			argString: `<prog> -n https://www.there.com`,
			ok:        false,
		},
		{ // 7
			argString:   `<prog> -v -s "hi" -z 100 -s "there" https://www.test.com`,
			SearchTerms: []string{"hi", "there"},
			Verbose:     true,
			BaseURL:     "https://www.test.com",
			ok:          true,
			BufferSize:  100,
		},
		{ // 8
			argString:   `<prog> -q 5 -s "hi" https://www.test.com`,
			SearchTerms: []string{"hi"},
			BaseURL:     "https://www.test.com",
			ok:          true,
			QuerySec:    5,
		},
		{ // 9
			argString:   `<prog> -t 3m20s -s "hi" https://www.test.com`,
			SearchTerms: []string{"hi"},
			BaseURL:     "https://www.test.com",
			ok:          true,
			Timeout:     "3m20s",
		},
		{ // 10
			argString:   `<prog> -s "hi" https://www.test.com`,
			SearchTerms: []string{"hi"},
			BaseURL:     "https://www.test.com",
			ok:          true,
			Timeout:     "2m", // default
		},
		{ // 11
			argString:   `<prog> -t "0s" -s "hi" https://www.test.com`,
			SearchTerms: []string{"hi"},
			BaseURL:     "https://www.test.com",
			ok:          true,
			Timeout:     "0s", // no timeout
		},
		{ // 12
			argString:   `<prog> -v -s "hi" -w 100 -s "there" https://www.test.com`,
			SearchTerms: []string{"hi", "there"},
			Verbose:     true,
			BaseURL:     "https://www.test.com",
			ok:          true,
			Workers:     100,
		},
		{ // 13
			argString:   `<prog> -v -s "hi" -x 100 -s "there" https://www.test.com`,
			SearchTerms: []string{"hi", "there"},
			Verbose:     true,
			BaseURL:     "https://www.test.com",
			ok:          true,
			HTTPWorkers: 100,
		},
		{ // 14
			argString:   `<prog> -v -t 1h20m10s -q 19 -s "hi" -z 5 -w 6 -x 7 -s "there" https://www.test.com`,
			SearchTerms: []string{"hi", "there"},
			Verbose:     true,
			BaseURL:     "https://www.test.com",
			ok:          true,
			BufferSize:  5,
			Workers:     6,
			HTTPWorkers: 7,
			QuerySec:    19,
			Timeout:     "1h20m10s",
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("test_%d", i), func(t *testing.T) {
			os.Args = strings.Fields(tt.argString)
			options, err := getOptions()
			var timeout time.Duration
			if tt.BufferSize == 0 {
				tt.BufferSize = LINKBUFFERSIZE
			}
			if tt.Workers == 0 {
				tt.Workers = GOWORKERS
			}
			if tt.HTTPWorkers == 0 {
				tt.HTTPWorkers = HTTPWORKERS
			}
			if tt.QuerySec == 0 {
				tt.QuerySec = HTTPRATESEC
			}
			if tt.Timeout != "" {
				var err error
				timeout, err = time.ParseDuration(tt.Timeout)
				if err != nil {
					t.Fatalf("invalid timeout %s %v", tt.Timeout, err)
				}
			} else {
				timeout, _ = time.ParseDuration("2m") // default
			}
			if err != nil && tt.ok {
				t.Errorf("unexpected error %v", err)
				t.Log(os.Args)
				return
			}
			if err == nil && !tt.ok {
				t.Errorf("unexpected lack of error for options %#v", options)
				t.Log(os.Args)
				return
			}
			if !tt.ok && err != nil {
				// fine
				return
			}
			if diff := cmp.Diff(options.SearchTerms, tt.SearchTerms); diff != "" {
				t.Errorf("searchterms mismatch (-want +got):\n%s", diff)
			}
			if got, want := options.Verbose, tt.Verbose; got != want {
				t.Errorf("verbose mismatch want %t got %t", got, want)
			}
			if got, want := options.BufferSize, tt.BufferSize; got != want {
				t.Errorf("link buffersize mismatch want %d got %d", got, want)
			}
			if got, want := options.Workers, tt.Workers; got != want {
				t.Errorf("workers mismatch want %d got %d", got, want)
			}
			if got, want := options.HTTPWorkers, tt.HTTPWorkers; got != want {
				t.Errorf("http workers mismatch want %d got %d", got, want)
			}
			if got, want := options.QuerySec, tt.QuerySec; got != want {
				t.Errorf("http rate/sec mismatch want %d got %d", got, want)
			}
			if got, want := options.Timeout, timeout; got != want {
				t.Errorf("timeout mismatch want %v got %v", got, want)
			}
			if got, want := options.Args.BaseURL, tt.BaseURL; got != want {
				t.Errorf("baseurl mismatch want %s got %s", got, want)
			}
		})
	}
}

func TestPrintResults(t *testing.T) {

	resulter := func() <-chan Result {
		r := make(chan Result, 5)
		r <- Result{
			url:     "http://example.com/nomatches",
			status:  200,
			matches: []SearchMatch{},
		}
		r <- Result{
			err: NonHTMLPageType,
		}
		r <- Result{
			referrer: "/referrer",
			url:      "http://example.com/403",
			status:   403,
			err:      StatusNotOk,
		}
		r <- Result{
			url:    "http://example.com/unknown",
			status: 200,
			err:    errors.New("unknown error"),
		}
		r <- Result{
			url:     "http://example.com/matches",
			status:  200,
			matches: []SearchMatch{{2, "hi"}, {99, "there"}},
		}
		close(r)
		return r
	}

	// redirect stdout
	var buf bytes.Buffer
	output = &buf

	options := Options{Verbose: true}
	options.Args.BaseURL = "https://example.com"
	printResults(options, resulter())

	// put back
	output = os.Stdout

	want := `
Commencing search of https://example.com:
http://example.com/nomatches
http://example.com/403
- status 403 (from /referrer)
http://example.com/unknown : error unknown error
http://example.com/matches
> line:   2 match: hi
> line:  99 match: there
processed 5 pages
`
	got := buf.String()
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}
