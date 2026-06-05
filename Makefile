.PHONY: build

build:
	go build -o bin/wtm .
	chmod +x bin/wtm
