# Diamondhands plugin

## Introduction
This is a plugin to support [DiamondHands](https://swap.diamondhands.technology/).

Internally it will use [Boltz plugin](../boltz/). It is simple to disable diamondhands plugin in compile time, but
if you want to disable Boltz (but still use Diamondhands) you will have to change init() in Boltz plugin to not register itself.
There is `--disableboltz` runtime option however (and also `--disablediamondhands`).

## Options

This section lists all configuration options. Most of them are hidden since they are changing implementation details not relevant for an average user. Even the ones which are mentioned in `--help` like
`--diamondhandsurl` do not have to be changed.

```
   --diamondhandszurl value   url of diamondhands api - empty means default - https://boltz.diamondhands.technology/api/
   --diamonhandsdatabase value      full path to database file (file will be created if it does not exist yet) (default: "/home/user/.bolt/diamondhands.db")
   --diamondhandszreferral value      boltz referral code (default: bolt-observer)
```

To learn more read: https://docs.bolt.observer/readme/liquidops/node-and-liquidity-automation
