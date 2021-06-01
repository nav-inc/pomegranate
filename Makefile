.PHONY: tests viewcoverage check ci

GOBIN ?= $(GOPATH)/bin

all: tests check

bin/pmg: pmg/pmg.go
	go build -o $@ $<

tests:
	go test . -mod=readonly

profile.cov:
	go test -coverprofile=$@ -mod=readonly

viewcoverage: profile.cov 
	go tool cover -html=$<

check: $(GOBIN)/golangci-lint
	$(GOBIN)/golangci-lint run
