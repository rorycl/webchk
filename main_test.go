package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetOptions(t *testing.T) {

	tests := []struct {
		argString   string
		SearchTerms []string
		Verbose     bool
		BaseURL     string
		ok          bool
	}{
		{
			argString: "<prog> -h",
			ok:        false, // actually osexits
		},
		{
			argString: `<prog> https://www.test.com`,
			ok:        false,
		},
		{
			argString:   `<prog> -s "hi" https://www.test.com`,
			SearchTerms: []string{"hi"},
			Verbose:     false,
			BaseURL:     "https://www.test.com",
			ok:          true,
		},
		{
			argString:   `<prog> -s "hi" -s "there" https://www.test.com`,
			SearchTerms: []string{"hi", "there"},
			Verbose:     false,
			BaseURL:     "https://www.test.com",
			ok:          true,
		},
		{
			argString:   `<prog> -v -s "hi" -s "there" https://www.test.com`,
			SearchTerms: []string{"hi", "there"},
			Verbose:     true,
			BaseURL:     "https://www.test.com",
			ok:          true,
		},
		{
			// no base url
			argString: `<prog> -v -s "hi" -s "there"`,
			ok:        false,
		},
		{
			// unknown flag
			argString: `<prog> -n https://www.there.com`,
			ok:        false,
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("test_%d", i), func(t *testing.T) {
			os.Args = strings.Fields(tt.argString)
			options, err := getOptions()
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
			url:    "http://example.com/403",
			status: 403,
			err:    StatusNotOk,
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
http://example.com/403 : status 403
http://example.com/unknown : error unknown error
http://example.com/matches
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
