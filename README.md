# webchk

A Go command to recursively search a website for case insensitive search
terms.

Future releases may make the number of concurrent http connections and
amount of concurrency tunable. The buffer of links to follow is
presently fixed at 1000; more than that will exit the program.

```
Usage:
  webchk -s searchterm [-s searchterm]... <baseurl>

Look for one or more searchterms (typically constrained between double
quotes) in a website starting at <baseurl>.

Application Arguments:

  BaseURL

Application Options:
  -s, --searchterm= search terms, can be specified more than once
  -v, --verbose     set verbose output

Help Options:
  -h, --help        Show this help message

Arguments:
  BaseURL:          base url to search
```

Example:

```
./webchk -v -s "quantum computer" https://slashdot.org | head -n 20

Commencing search of https://slashdot.org:
https://slashdot.org
https://slashdot.org
> line: 1396 match: quantum computer
> line: 1447 match: quantum computer
> line: 1449 match: quantum computer
https://slashdot.org/archive.pl
https://slashdot.org/archive.pl
> line: 1151 match: quantum computer
https://slashdot.org/blog
https://slashdot.org/faq
https://slashdot.org/faq/slashmeta.shtml
https://slashdot.org/hof.shtml
https://slashdot.org/index2.pl
https://slashdot.org/index2.pl
> line: 1396 match: quantum computer
> line: 1447 match: quantum computer
> line: 1449 match: quantum computer
https://slashdot.org/jobs
```


Licensed under the [MIT Licence](LICENCE).
