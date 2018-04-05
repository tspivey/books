VERSION = $(shell git describe --always --long --dirty)
PACKAGE = github.com/tspivey/books
BUILDFLAGS = -ldflags "-X $(PACKAGE).Version=$(VERSION)"
ifdef WINDIR
	CURRENT = books.exe
else
	CURRENT = books
endif

.PHONY: all clean install windows linux freebsd darwin

books: current

all: windows linux freebsd darwin current

windows: bin
	GOOS=windows GOARCH=amd64 go build $(BUILDFLAGS) -o bin/books-windows-amd64.exe cmd/books/main.go

linux: bin
	GOOS=linux GOARCH=amd64 go build $(BUILDFLAGS) -o bin/books-linux-amd64 cmd/books/main.go

freebsd: bin
	GOOS=freebsd GOARCH=amd64 go build $(BUILDFLAGS) -o bin/books-freebsd-amd64 cmd/books/main.go

darwin: bin
	GOOS=darwin GOARCH=amd64 go build $(BUILDFLAGS) -o bin/books-darwin-amd64 cmd/books/main.go

current: bin
	go build $(BUILDFLAGS) -o bin/$(CURRENT) cmd/books/main.go

bin:
	mkdir bin

install:
	go install $(BUILDFLAGS) $(PACKAGE)/cmd/books

clean:
	rm -rf bin