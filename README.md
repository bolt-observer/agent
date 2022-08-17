# Agent

Agent is a piece of software that is able to run on lightning node and report various data from it.

Currently we have:
* balance-agent: an agent that can report channel balance information

Internally we have:
* channelchecker: the abstraction for checking all channels
* lightning_api: an abstraction around LND API

We depend on github.com/bolt-observer/go_common for common utilities.

## How to

```
go env -w GOPRIVATE=github.com/bolt-observer/go_common
```

Do:
```
go get -v github.com/lightningnetwork/lnd@v0.15.0-beta
```
or else you get an ancient version and everything breaks (should be good due to go.mod/sums now).
