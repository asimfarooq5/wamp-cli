deps:
	go get github.com/s-things/wick/cmd/wick

build:
	go build github.com/s-things/wick/cmd/wick

test:
	go test github.com/s-things/wick/cmd/wick -v
	go test github.com/s-things/wick/core -v

clean:
	rm -f wick

lint:
	golangci-lint run
