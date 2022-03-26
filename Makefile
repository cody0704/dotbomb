NAME="dotbomb"
VERSION ?= $(shell git describe --tags --always || git rev-parse --short HEAD)

compile:
	echo "Compiling for every OS and Platform"
	GOOS=freebsd GOARCH=amd64 go build -o ./bin/$(NAME)_$(VERSION)_freebsd-amd64 -ldflags '-X "main.versionID=$(VERSION)"' ./cmd/dotbomb
	GOOS=linux GOARCH=amd64 go build -o ./bin/$(NAME)_$(VERSION)_linux-amd64 -ldflags '-X "main.versionID=$(VERSION)"'  ./cmd/dotbomb
	GOOS=darwin GOARCH=amd64 go build -o ./bin/$(NAME)_$(VERSION)_darwin-amd64 -ldflags '-X "main.versionID=$(VERSION)"'  ./cmd/dotbomb
	GOOS=windows GOARCH=amd64 go build -o ./bin/$(NAME)_$(VERSION)_windows-amd64 -ldflags '-X "main.versionID=$(VERSION)"'  ./cmd/dotbomb