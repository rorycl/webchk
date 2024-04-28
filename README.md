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


Licensed under the [MIT Licence](LICENCE).
