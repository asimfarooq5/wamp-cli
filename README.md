# WICK
CLI tool to make WAMP RPCs and PubSub. Useful for developing WAMP Components and their testing.

## How to install
On Linux use snapd
```shell
sudo snap install wick
```
On macOS use brew
```shell
brew tap codebasepk/wick https://github.com/codebasepk/wick
brew install wick
```

## How to build
```bash
git clone git@github.com:codebasepk/wick.git
cd wick
go get github.com/codebasepk/wick/cmd/wick
go build github.com/codebasepk/wick/cmd/wick
./wick
```
