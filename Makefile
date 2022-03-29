NAME="dotbomb"
VERSION ?= $(shell git describe --tags --always || git rev-parse --short HEAD)

compile:
	echo "Compiling for every OS and Platform"
	GOOS=freebsd GOARCH=amd64 go build -o ./bin/$(NAME) -ldflags '-X "main.versionID=$(VERSION)"' ./cmd/dotbomb
	zip ./bin/$(NAME)_$(VERSION)_freebsd-amd64.zip ./bin/$(NAME)
	rm -rf ./bin/$(NAME)

	GOOS=linux GOARCH=amd64 go build -o ./bin/$(NAME) -ldflags '-X "main.versionID=$(VERSION)"'  ./cmd/dotbomb
	zip ./bin/$(NAME)_$(VERSION)_linux-amd64.zip  ./bin/$(NAME)
	rm -rf ./bin/$(NAME)

	GOOS=darwin GOARCH=amd64 go build -o ./bin/$(NAME) -ldflags '-X "main.versionID=$(VERSION)"'  ./cmd/dotbomb
	zip ./bin/$(NAME)_$(VERSION)_darwin-amd64.zip ./bin/$(NAME)
	rm -rf ./bin/$(NAME)

	GOOS=windows GOARCH=amd64 go build -o ./bin/$(NAME).exe -ldflags '-X "main.versionID=$(VERSION)"'  ./cmd/dotbomb
	zip ./bin/$(NAME)_$(VERSION)_windows-amd64.zip ./bin/$(NAME).exe
	rm -rf ./bin/$(NAME).exe