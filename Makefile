GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

BINARY_NAME=go-books
RELEASE_BUILDS=bins

LDFLAGS=-ldflags="-s -w"
GCFLAGS=-gcflags="-m -l -B"

CGO_ENABLED=1

CGO_CPPFLAGS=-I/opt/homebrew/Cellar/leptonica/1.84.1/include -I/opt/homebrew/Cellar/tesseract/5.4.1_1/include
CGO_LDFLAGS=-L/opt/homebrew/Cellar/leptonica/1.84.1/lib -L/opt/homebrew/Cellar/tesseract/5.4.1_1/lib

all: build

build:
	CGO_ENABLED=$(CGO_ENABLED) CGO_CPPFLAGS="$(CGO_CPPFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" $(GOBUILD) $(LDFLAGS) $(GCFLAGS) -o $(BINARY_NAME) .

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf *-*/

run: build
	./$(BINARY_NAME)

deps:
	$(GOGET) -v -t -d ./...

.PHONY: all build build-prod clean run deps