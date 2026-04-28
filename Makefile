NAME="dotbomb"
VERSION ?= $(shell git describe --tags --always || git rev-parse --short HEAD)

linux:
	echo "Compiling for Linux"
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o ./bin/$(NAME) -ldflags '-X "main.versionID=$(VERSION)"' ./cmd/dotbomb
	cd ./bin && zip $(NAME)_$(VERSION)_linux-amd64.zip $(NAME)
	rm -rf ./bin/$(NAME)

windows:
	echo "Compiling for Windows"
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -o ./bin/$(NAME).exe -ldflags '-X "main.versionID=$(VERSION)"' ./cmd/dotbomb
	cd ./bin && zip $(NAME)_$(VERSION)_windows-amd64.zip $(NAME).exe
	rm -rf ./bin/$(NAME).exe

drawin:
	echo "Compiling for macOS"
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o ./bin/$(NAME) -ldflags '-X "main.versionID=$(VERSION)"' ./cmd/dotbomb
	cd ./bin && zip $(NAME)_$(VERSION)_darwin-amd64.zip $(NAME)
	rm -rf ./bin/$(NAME)