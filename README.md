# Agent

Agent is a piece of software that is able to run on lightning node (we provide raspberry builds too) and report various data from it.

## Contents:

Currently we have:

### balance-agent

utility to monitor channel balances

The usage should be pretty self expanatory:

```
NAME:
   balance-agent - Utility to monitor channel balances

USAGE:
   balance-agent [global options] command [command options] [arguments...]

VERSION:
   v0.0.22

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
   --interval value           interval to poll - 10s, 1m, 10m or 1h (default: "10s")
   --private                  report private data as well (default: false)
   --preferipv4               If you have the choice between IPv6 and IPv4 prefer IPv4 (default: false)
   --verbosity value          log level for V logs (default: 0)
   --help, -h                 show help
   --version, -v              print the version
```

It tries the best to have sane defaults so you can just start it up on your node without further hassle.
If you have a strange lnd dir (`/storage` in the example) you might need:
```
./balance-agent --lnddir /storage/lnd/ --tlscertpath /storage/lnd/data/secrets/lnd.cert
```
but that should be it.

Only thing that is mandatory is apikey which can also be provided through `API_KEY` environment variable.

By default it will try to communicate with LND using local gRPC connection and `readonly.macaroon`.

You can use `--userest` to start using REST API (default is as mentioned gRPC) - in that case `--rpcserver` can be complete URL to the endpoint.

Of course you can also start the agent on a remote node and just point it to the correct endpoint, but since this is not the default
use cases flags `--userest` and `--rpcserver` are not advertied in help.

Flag `--private` means whether you want to report data for private ("unannounced") channels too (when true). The default is false.

## Components

Internally we use:
* [channelchecker](./channelchecker): an abstraction for checking all channels
* [nodeinfo](./nodeinfo): this can basically report `lncli getnodeinfo` for your node  - it is used by the agent so we have a full view of node info & channels
* [checkermonitoring](./checkermonitoring): is used for reporting metrics via Graphite (not used directly in balance-agent here)
* [lightning_api](./lightning_api): an abstraction around lightning node API (that furthermore heavily depends on common code from [lnd](https://github.com/lightningnetwork/lnd))

## Dependencies

Currently the dependencies are still in private git repositories so you need to do something like:

```
go env -w GOPRIVATE=github.com/bolt-observer/go_common
git config url."git@github.com:".insteadOf "https://github.com/"
```

## Verifying the Release

```
curl https://raw.githubusercontent.com/bolt-observer/agent/main/scripts/keys/fiksn.asc | gpg --import
gpg --verify manifest-v0.0.3.txt.asc manifest-v0.0.3.txt
```

and you should see:
```
gpg:                using RSA key F4B8B3B59C1E5AA39A1B9636E897355718E1DBF4
gpg: Good signature from "Gregor Pogacnik <gregor@bolt.observer>" [ultimate]
```

## Troubleshooting

Do:
```
go get -v github.com/lightningnetwork/lnd@v0.15.0-beta
```
or else you get an ancient version and everything breaks (should be good due to go.mod/sums now).
