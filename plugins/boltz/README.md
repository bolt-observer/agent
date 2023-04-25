# Boltz plugin

## Introduction
This is a plugin to do submarine swaps using [Boltz.Exchange](https://boltz.exchange/).
Submarine swap is a special type of atomic swap between on-chain and off-chain funds. This means it does not bear counterparty risk. Either the swap will go through completely or nothing will happen.

### Auto swap mode

The preferred way is to get swap requests directly from [bolt.observer](https://bolt.observer) backend. This way your channels are kept balanced automatically.
You need to run agent with `--actions` for this mode (and other workflow actions) to be enabled. There is also a `--dryrun` flag so it will attempt to
do swaps but never actually use any funds.

In this mode you configure:
* desired node inbound liquidity
* desired node outbound liquidity
* desired channel outbound liqudity

and plugin will automatically calculate how big the swap should be and if it needs to do more than one in order to achieve desired targets.

`--zeroconf` can be used to trust 0-confirmation transactions in the mempool and wait until the transaction is confirmed in a block.
`--minswapsats` and `--maxswapsats` are for your control of how big swaps are done (you can set the values to 0 and then boltz limits will apply)

It might do up to `--maxswapattempts` individual swaps (currently this is set to a default of 3 for safety reasons). But this is an implementation detail.

The real limit you are interested in are the fees: `--maxfeepercentage` is 5% by default and basically until we have spent less than that for the rebalancing we
go on and create another swap to bring liquidity in line with desired settings. This setting is for one rebalancing event that can cause multiple swaps.

If the workflow action is triggered again (e.g., liquidity goes above target and then again below) then another rebalancing is attempted and it again has the full 5% "allowance".

Global flags are used in a meanigful way, for instance if `--network testnet` then `--boltzurl` is changed to point to testnet boltz URL.

### CLI mode

```
NAME:
   bolt-agent boltz - interact with boltz plugin

USAGE:
   bolt-agent boltz command [command options] [arguments...]

COMMANDS:
   submarineswap         invoke submarine swap aka swap-in (on-chain -> off-chain)
   reversesubmarineswap  invoke reverse submarine swap aka swap-out (off-chain -> on-chain)
   generaterefund        generate refund file to be used directly on boltz exchange GUI
   dumpmnemonic          print master secret as mnemonic phrase (dangerous)
   setmnemonic           set a new secret from mnemonic phrase (dangerous)

OPTIONS:
   --help, -h  show help
```

Plugin can be executed in standalone CLI mode too which enables you to do submarine or reverse submarine swaps on your node immediately.

You can manually invoke a swap like this:

```
bolt-agent boltz submarineswap --sats 100000
```

ID is reported so you can continue doing a swap also if application died in the middle.
Just add `--id -1337` where -1337 is the ID reported and swap will be resumed from where it stopped before.

`generaterefund` generates a JSON file that can be used on Boltz GUI

#### Mnemonic

`dumpmnemonic` and `setmnemonic` should not be used, but they allow you to change entropy (also stored in database). All secrets (like keys and preimages are generated from it, so you should never write this down anywere!).
The command can be useful if agent died in the middle of a swap and you want to resume the swap using CLI mode on another node (you just `dumpmnemonic` and import entropy on new node using `setmnemonic`).
Then same ID should result in same secrets and you can for instance use `submarineswap` on second node to finish the swap (or get a refund).

## Options

This section lists all configuration options. Most of them are hidden since they are changing implementation details not relevant for an average user. Even the ones which are mentioned in `--help` like
`--boltzurl` do not have to be changed.

```
   --boltzurl value           url of boltz api - empty means default - https://boltz.exchange/api or https://testnet.boltz.exchange/api
   --boltzdatabase value      full path to database file (file will be created if it does not exist yet) (default: "/home/user/.bolt/boltz.db")
   --boltzreferral value      boltz referral code (default: bolt-observer)
   --zeroconf                 enable zeroconfirmation for swaps (default: true)
   --maxfeepercentage value   maximum fee in percentage that is still acceptable (default: 5)
   --maxswapsats value        maximum swap to perform in sats (default: 0)
   --minswapsats value        minimum swap to perform in sats (default: 0)
   --maxswapattempts value    maximum number of swaps to bring liquidity to desired state (default 20)
   --defaultswapsats value    default swap to perform in sats (default: 0)
```

To learn more read: https://docs.bolt.observer/readme/liquidops/node-and-liquidity-automation
