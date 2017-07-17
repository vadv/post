SOURCEDIR=./src
SOURCES := $(shell find $(SOURCEDIR) -name '*.go')

VERSION := $(shell git describe --abbrev=0 --tags)
SHA := $(shell git rev-parse --short HEAD)

GOPATH ?= /usr/local/go
GOPATH := ${CURDIR}:${GOPATH}
export GOPATH

all: ./bin/post

./bin/post: $(SOURCES)
	go build -o ./bin/post -ldflags "-X main.BuildVersion=$(VERSION)-$(SHA)" $(SOURCEDIR)/cmd/main.go

tar: clean
	mkdir -p rpm/SOURCES
	tar --transform='s,^\.,post-$(VERSION),'\
		-czf rpm/SOURCES/post-$(VERSION).tar.gz .\
		--exclude=rpm/SOURCES

test:
	go test -x -v storage

docker: submodule_check tar
	cp -a $(CURDIR)/rpm /build
	cp -a $(CURDIR)/rpm/SPECS/post.spec /build/SPECS/post-$(VERSION).spec
	sed -i 's|%define version unknown|%define version $(VERSION)|g' /build/SPECS/post-$(VERSION).spec
	chown -R root:root /build
	rpmbuild -ba --define '_topdir /build'\
		/build/SPECS/post-$(VERSION).spec

clean:
	rm -f rpm-tmp.*
	rm -rf pkg

.DEFAULT_GOAL: all

include Makefile.git
