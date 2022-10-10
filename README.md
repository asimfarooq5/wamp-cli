# WICK
CLI tool to make WAMP RPCs and PubSub. Useful for developing WAMP Components and their testing.

## CLI
The basic command-line interface looks like
```shell
om26er@P1:~/Documents$ wick --help-long
usage: wick [<flags>] <command> [<args> ...]

Flags:
      --help                     Show context-sensitive help (also try --help-long and --help-man).
      --url="ws://localhost:8080/ws"  
                                 WAMP URL to connect to.
      --realm="realm1"           The WAMP realm to join.
      --authmethod=anonymous     The authentication method to use.
      --authid=AUTHID            The authid to use, if authenticating.
      --authrole=AUTHROLE        The authrole to use, if authenticating.
      --secret=SECRET            The secret to use in Challenge-Response Auth.
      --private-key=PRIVATE-KEY  The ed25519 private key hex for cryptosign.
      --ticket=TICKET            The ticket when using ticket authentication.
      --serializer=json          The serializer to use.
      --profile=PROFILE          Get details from in '$HOME/.wick/config'.For default section use 'DEFAULT'.
      --debug                    Enable debug logging.
  -v, --version                  Show application version.

Commands:
  help [<command>...]
    Show help.


  join-only [<flags>]
    Start wamp session.

    --parallel=1     Start requested number of wamp sessions.
    --concurrency=1  Start wamp session concurrently. Only effective when called with --parallel.
    --time           Log session join time
    --keepalive=0    Interval between websocket pings.

  subscribe [<flags>] <topic>
    Subscribe a topic.

    -o, --option=OPTION ...  Subscribe option. (May be provided multiple times)
        --details            Print event details.
        --event-count=0      Wait for a given number of events and exit.
        --time               Log time to join session and subscribe a topic.
        --concurrency=1      Subscribe to topic concurrently. Only effective when called with --parallel.
        --parallel=1         Start requested number of wamp sessions.
        --keepalive=0        Interval between websocket pings.

  publish [<flags>] <topic> [<args>...]
    Publish to a topic.

    -k, --kwarg=KWARG ...    Provide the keyword arguments.
    -o, --option=OPTION ...  Publish option. (May be provided multiple times)
        --repeat=1           Publish to the topic for the provided number of times.
        --time               Log publish return time.
        --delay=0            Provide the delay in milliseconds.
        --concurrency=1      Publish to the topic concurrently. Only effective when called with --repeat and/or --parallel.
        --parallel=1         Start requested number of wamp sessions
        --keepalive=0        Interval between websocket pings.

  register [<flags>] <procedure> [<command>]
    Register a procedure.

        --delay=DELAY        Register procedure after delay.(in milliseconds)
        --invoke-count=INVOKE-COUNT  
                             Leave session after its called requested times.
    -o, --option=OPTION ...  Procedure registration option. (May be provided multiple times)
        --time               Log time to join session and register procedure.
        --concurrency=1      Register procedure concurrently. Only effective when called with --parallel.
        --parallel=1         Start requested number of wamp sessions.
        --keepalive=0        Interval between websocket pings.

  call [<flags>] <procedure> [<args>...]
    Call a procedure.

    -k, --kwarg=KWARG ...    Provide the keyword arguments.
        --time               Log call return time.
        --repeat=1           Call the procedure for the provided number of times.
        --delay=0            Provide the delay in milliseconds.
    -o, --option=OPTION ...  Procedure call option. (May be provided multiple times)
        --concurrency=1      Make concurrent calls without waiting for the result for each to return. Only effective when called with --repeat and/or --parallel.
        --parallel=1         Start requested number of wamp sessions.
        --keepalive=0        Interval between websocket pings.
```
### Call a procedure
```shell
wick --url ws://localhost:8080/ws --realm realm1 call foo.bar
````

### Publish an event
```shell
wick --url ws://localhost:8080/ws --realm realm1 publish foo.bar arg1 arg2 --kwarg key=value --kwarg key2=value2
```

### Environment variables
Wick supports reading environment variables for all the WAMP config (realm, URL, authid, private-key...).
This is makes it effective to integrate in CI scenarios.
```shell
export WICK_URL=ws://localhost:8080/ws
export WICK_REALM=something
wick call foo.bar
```

### Supported Environment Variables
These are self-explanatory.
```shell
WICK_URL
WICK_REALM
WICK_AUTHMETHOD
WICK_AUTHID
WICK_AUTHROLE
WICK_SECRET
WICK_PRIVATE_KEY
WICK_TICKET
WICK_SERIALIZER
```


## How to install
On Linux use snapd
```shell
sudo snap install wick --classic
```
On macOS use brew
```shell
brew tap s-things/wick https://github.com/s-things/wick
brew install wick
```

## How to build
```bash
git clone git@github.com:s-things/wick.git
cd wick
go get github.com/s-things/wick/cmd/wick
go build github.com/s-things/wick/cmd/wick
./wick
```
