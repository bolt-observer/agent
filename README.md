# Agent

Agent is a piece of software that is able to run on lightning node (we provide raspberry builds too) and report various data from it.

## Contents:

Currently we have:

### balance-agent: an agent that can report channel balance information

The usage should be pretty self expanatory:

```
NAME:
   balance-agent - Utility to monitor channel balances

USAGE:
   balance-agent [global options] command [command options] [arguments...]

VERSION:
   0.0.1

COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --apikey value             api key
   --rpcserver value          host:port of ln daemon (default: "localhost:10009")
   --lnddir value             path to lnd's base directory (default: "/home/user/lnd")
   --tlscertpath value        path to TLS certificate (default: "/home/user/lnd/tls.cert")
   --chain value, -c value    the chain lnd is running on e.g. bitcoin (default: "bitcoin")
   --network value, -n value  the network lnd is running on e.g. mainnet, testnet, etc. (default: "mainnet")
   --macaroonpath value       path to macaroon file
   --allowedentropy value     allowed entropy in bits for channel balances (default: 64)
   --interval value           interval to poll - 10s, 1m or 1h (default: "10s")
   --private                  report private channels as well
   --help, -h                 show help
   --version, -v              print the version
```

It tries the best to have sane defaults so you can just start it up on your node without further hassle.
If you have a strange lnd dir you might need:
```
./balance-agent --lnddir /storage/lnd/ --tlscertpath /storage/lnd/data/secrets/lnd.cert
```
but that should be it.

Only thing that is mandatory is apikey which can also be provided through `API_KEY` environment variable.

By default it will try to communicate with LND using local gRPC connection and `readonly.macaroon`.

You can use `--userest` to start using REST API (default is as mentioned gRPC) - in that case `--rpcserver` can be complete URL to the endpoint.

Of course you can also start the agent on a remote node and just point it to the correct endpoints.

Internally we use:
* [channelchecker](./channelchecker): an abstraction for checking all channels
* [lightning_api](./lightning_api): an abstraction around lightning node API (that furthermore heavily depends on common code from [lnd](https://github.com/lightningnetwork/lnd))

## Dependencies

Currently the dependencies are still in private git repositories so you need to do something like:

```
go env -w GOPRIVATE=github.com/bolt-observer/go_common
git config url."git@github.com:".insteadOf "https://github.com/"
```

## Troubleshooting

Do:
```
go get -v github.com/lightningnetwork/lnd@v0.15.0-beta
```
or else you get an ancient version and everything breaks (should be good due to go.mod/sums now).
