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
encounters a "too many requests" 429 response or if it times out. The
'querysec' parameter is set to 10 queries/sec by default to avoid
overloading the target system.

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
rory:~/src/go-webchk$ ./webchk -v -s "amazing" https://slashdot.org | head -n 20 

Commencing search of https://slashdot.org:
https://slashdot.org
https://slashdot.org/jobs
https://slashdot.org/faq
https://slashdot.org/my/login
https://slashdot.org/faq/slashmeta.shtml
https://slashdot.org/hof.shtml
https://slashdot.org/my/newuser
- status 403 (from https://slashdot.org)
https://slashdot.org/index2.pl
https://slashdot.org/my/mailpassword
https://slashdot.org/blog
> line: 3229 match: amazing
https://slashdot.org/archive.pl
https://slashdot.org/newsletter
https://slashdot.org/poll/3251/will-bytedance-be-forced-to-divest-tiktok
https://slashdot.org/polls
https://slashdot.org/popular
https://slashdot.org/story/22/05/26/1748248/broadcom-to-acquire-vmware-in-massive-61-billion-deal
```


Licensed under the [MIT Licence](./LICENCE).
