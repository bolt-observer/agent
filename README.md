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
   v0.0.41

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --allowedentropy value      allowed entropy in bits for channel balances (default: 64)
   --apikey value              api key
   --channel-whitelist value   Path to file containing a whitelist of channels
   --interval value            interval to poll - 10s, 1m, 10m or 1h (default: "10s")
   --lnddir value              path to lnd's base directory (default: "/home/user/.lnd")
   --macaroonpath value        path to macaroon file
   --private                   report private data as well (default: false)
   --preferipv4                If you have the choice between IPv6 and IPv4 prefer IPv4 (default: false)
   --rpcserver value           host:port of ln daemon (default: "localhost:10009")
   --tlscertpath value         path to TLS certificate (default: "/home/user/.lnd/tls.cert")
   --channel-whitelist value   Path to file containing a whitelist of channels
   --verbosity value           log level for V logs (default: 0)
   --help, -h                  show help
   --version, -v               print the version
```

It tries the best to have sane defaults so you can just start it up on your node without further hassle.
If you have a strange lnd dir (`/storage` in the example) you might need:
```
balance-agent --lnddir /storage/lnd/ --tlscertpath /storage/lnd/data/secrets/lnd.cert
```
but that should be it.

Only thing that is mandatory is apikey which can also be provided through `API_KEY` environment variable.

By default it will try to communicate with LND using local gRPC connection and `readonly.macaroon`.

You can use `--userest` to start using REST API (default is as mentioned gRPC) - in that case `--rpcserver` can be complete URL to the endpoint.

Of course you can also start the agent on a remote node and just point it to the correct endpoint, but since this is not the default
use cases flags `--userest` and `--rpcserver` are not advertied in help.

Flag `--private` means whether you want to report data for private ("unannounced") channels too (when true). The default is false.

For more information about what exactly is reported read [DATA.md](./DATA.md).

## Install

We provide a standalone statically compiled binary. The targets are `linux` (amd64 build for GNU/Linux), `rasp` (aarch64 build for GNU/Linux) suitable for installation on your Raspberry PI and `darwin` (amd64 build for MacOS).
If you need some different architecture that is no problem except that you will have to build the binaries yourself (so you will need to install Golang). You can also open an issue / pull request to extend the defaults (or provide better naming).

Installation steps:

* fetch latest revision from https://github.com/bolt-observer/agent/releases

```
wget https://github.com/bolt-observer/agent/releases/download/v0.0.41/balance-agent-v0.0.41-linux.zip https://github.com/bolt-observer/agent/releases/download/v0.0.41/manifest-v0.0.41.txt.asc https://github.com/bolt-observer/agent/releases/download/v0.0.41/manifest-v0.0.41.txt
```

* verify integrity

```
wget -qO- https://raw.githubusercontent.com/bolt-observer/agent/main/scripts/keys/fiksn.asc | gpg --import
gpg --verify manifest-v0.0.41.txt.asc manifest-v0.0.41.txt
```

and you should see:
```
gpg:                using RSA key F4B8B3B59C1E5AA39A1B9636E897355718E1DBF4
gpg: Good signature from "Gregor Pogacnik <gregor@bolt.observer>" [ultimate]
```

* unpack the compressed binary

```
unzip balance-agent-v0.0.41-linux.zip
```

* copy the binary to a common place

```
cp balance-agent-v0.0.41-linux /usr/local/bin/balance-agent
```

* start the binary

```
balance-agent -apikey changeme
```

* you may want to run the agent as a systemd service (we have a simple systemd template [here](./balance-agent.service))

You need to do that only once:

```
wget https://raw.githubusercontent.com/bolt-observer/agent/main/balance-agent.service
cp balance-agent.service /etc/systemd/system/balance-agent.service
$EDITOR /etc/systemd/system/balance-agent.service # to change API_KEY
systemctl daemon-reload
systemctl enable balance-agent
systemctl start balance-agent
```

## Docker

You can use the docker image from GitHub:

Usage:

```
docker run -v /tmp:/tmp -e API_KEY=changeme ghcr.io/bolt-observer/agent:v0.0.41
```

## Filtering on agent side

You can limit what channnels are reported using `--channel-whitelist` option. It specifies a local file path to be used as a whitelist of what channels to report.
When adding `--private` to `--channel-whitelist` this means every private channel AND whatever is listed in the file. There is also `--public` to allow all public channels.
Using the options without `--channel-whitelist` makes no sense since by default all public channels are reported however it has to be explicit with `--channel-whitelist` in order
to automatically allow all public channels (beside what is allowed through the file).
If you set `--private` and `--public` then no matter what you add to the `--channel-whitelist` file everything will be reported.

The file looks like this:

```
# Comments start with a # character
# You can list pubkeys for example:
0288037d3f0bdcfb240402b43b80cdc32e41528b3e2ebe05884aff507d71fca71a # bolt.observer
# which means any channel where peer pubkey is this
# or you can specify a specific short channel id e.g.,
759930760125546497
# too, invalid lines like
whatever
# will be ignored (and logged as a warning, aliases also don't work!)
# Validity for channel id is not checked (it just has to be numeric), thus:
1337
# is perfectly valid (altho it won't match and thus allow the reporting of
# any additional channel).
# Empty files means nothing - in whitelist context: do not report anything.
```

## Components

Internally we use:
* [nodedata](./nodedata): an abstraction for running the periodic checks and reporting balance related changes
* [filter](./filter): this is used to filter specific channels on the agent side
* [checkermonitoring](./checkermonitoring): is used for reporting metrics via Graphite (not used directly in balance-agent here)
* [lightning_api](./lightning_api): an abstraction around lightning node API (that furthermore heavily depends on common code from [lnd](https://github.com/lightningnetwork/lnd))

## Dependencies

This code depends on some [common code](https://github.com/bolt-observer/go_common).

## Troubleshooting

Do:
```
go get -v github.com/lightningnetwork/lnd@v0.15.4-beta
```
or else you get an ancient version and everything breaks (should be good due to go.mod/sums now).
