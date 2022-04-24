deps:
	go get github.com/s-things/wick/cmd/wick

build:
	go build github.com/s-things/wick/cmd/wick

run:
	./wick

clean:
	rm -f wick
