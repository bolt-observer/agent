# Boltz plugin

Boltz plugin obtains targets through actions API and invokes submarine swaps between on-chain and off-chain using [Boltz.Exchange](https://boltz.exchange/)
to end up with desired inbound or outbound liquidity. A single swap will always be between `--minswapsats` and `--minswapsats`. The option
`--maxfeepercentage` additionally allows you to limit how much is being spent on the swap (relative to amount of sats being swapped).

In order to do submarine swaps you must have an on-chain balance.

## Options

```
   --boltzurl value           url of boltz api (default: "https://boltz.exchange/api")
   --boltzdatabase value      full path to database file (file will be created if it does not exist yet) (default: "/home/user/.bolt/boltz.db")
   --maxfeepercentage value   maximum fee in percentage that is still acceptable (default: 5)
   --maxswapsats value        maximum swap to perform in sats (default: 1000000)
   --minswapsats value        minimum swap to perform in sats (default: 100000)
   --defaultswapsats value    default swap to perform in sats (default: 100000)
   --zeroconf                 enablle zeroconfirmation for swaps (default: true)
```
