# webchk

A Go program to recursively search a website for search terms.

```
Usage:
  webchk -s "searchterm" [-s "searchterm"]... <baseurl>

Look for one or more case-insensitive search terms (typically
constrained between double quotes) in a website starting at <baseurl>.

The timeout should be specified as a go time.ParseDuration string, for
example "1m30s". For no timeout, use a negative duration or "0s".

The program will exit early if the link buffer becomes full, if it
encounters a "too many requests" 429 response or if it times out.

Application Arguments:

  BaseURL

Application Options:
  -s, --searchterm=  search terms, can be specified more than once
  -v, --verbose      set verbose output
  -q, --querysec=    queries per second (default: 10)
  -t, --timeout=     program timeout (default: 2m)
  -z, --buffersize=  size of links buffer (default: 2500)
  -w, --workers=     number of goroutine workers (default: 8)
  -x, --httpworkers= number of http workers (default: 8)

Help Options:
  -h, --help         Show this help message

Arguments:
  BaseURL:           base url to search

```

Build the program using `make build` or `go build` (with go >= 1.22), or
download a binary from [Releases](./releases/).

Example:

```
$ ./webchk -v -s "espionage" https://slashdot.org | head -n 20

Commencing search of https://slashdot.org:
https://slashdot.org
https://slashdot.org/jobs
https://slashdot.org/faq/slashmeta.shtml
https://slashdot.org/hof.shtml
https://slashdot.org/index2.pl
https://slashdot.org/my/newuser : status 403
https://slashdot.org/faq
https://slashdot.org/login.pl
https://slashdot.org/my/mailpassword
https://slashdot.org/my/login
https://slashdot.org/newsletter
https://slashdot.org/popular
https://slashdot.org/popular
> line: 2522 match: espionage
> line: 2524 match: espionage
https://slashdot.org/archive.pl
https://slashdot.org/archive.pl
> line: 1352 match: espionage
```


Licensed under the [MIT Licence](./LICENCE).
