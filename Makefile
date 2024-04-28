# Based on the Example from Joel Homes, author of "Shipping Go" at
# https://github.com/holmes89/hello-api/blob/main/ch10/Makefile

#  https://stackoverflow.com/a/54776239
SHELL := /bin/bash
GO_VERSION := 1.22  # <1>
COVERAGE_AMT := 75  # should be 80
HEREGOPATH := $(shell go env GOPATH)
CURDIR := $(shell pwd)

build:
	go test . && echo "---ok---" && go build .

build-many:
	bin/builder.sh . webchk

del-bin:
	rm ./webchk

test:
	go test . -coverprofile=coverage.out

coverage-verbose:
	go tool cover -func coverage.out | tee cover.rpt

coverage-ok:
	cat cover.rpt | grep "total:" | awk '{print ((int($$3) > ${COVERAGE_AMT}) != 1) }'

cover-report:
	go tool cover -html=coverage.out -o cover.html

clean:
	rm $$(find ${CURDIR} -name "*cover*html" -or -name "*cover.rpt" -or -name "*coverage.out")

check: check-format check-vet test coverage-verbose coverage-ok cover-report lint 

check-format: 
	test -z $$(go fmt ./...)

check-vet: 
	test -z $$(go vet ./...)

testme:
	echo $(HEREGOPATH)

install-lint:
	# https://golangci-lint.run/usage/install/#local-installation to GOPATH
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(HEREGOPATH)/bin v1.57.2
	# report version
	${HEREGOPATH}/bin/golangci-lint --version

lint:
	# golangci-lint run -v ./... 
	${HEREGOPATH}/bin/golangci-lint run ./... 

module-update-tidy:
	go get -u ./...
	go mod tidy

