# What kind of data does the agent report?

You can always check by running it with following flags, which forces a dry-run (without sending anything to the server).

```
--url "" --verbosity 2
```

Every report contains basic details: `poll_interval`, `allowed_entropy`, `allow_private_channels`, `chain`, `network` and `pubkey`.

Besides it can contain `node_details` in a JSON format similar to [lnd getnodeinfoi](https://lightning.engineering/api-docs/api/lnd/lightning/get-node-info).
Additional fields are `node_version`, `is_synced_to_chain` and `is_synced_to_graph`, `onchain_balance_total` and `onchain_balance_total`. Node details will be reported just first time and if anything changes later on (during periodic polling).
Using `--noonchainbalance` it is possible to disable reporting on-chain balance.

```
{
  "poll_interval": "10s",
  "allowed_entropy": 64,
  "allow_private_channels": true,
  "chain": "bitcoin",
  "network": "mainnet",
  "pubkey": "0288037d3f0bdcfb240402b43b80cdc32e41528b3e2ebe05884aff507d71fca71a",
  "timestamp": 1673781849,
  "node_details": {
    "node_version": "lnd-0.15.5-beta commit=",
    "is_synced_to_chain": true,
    "is_synced_to_graph": true,
    "onchain_balance_total": 1337,
    "onchain_balance_confirmed": 1336,
    "node": {
       "pub_key": "0288037d3f0bdcfb240402b43b80cdc32e41528b3e2ebe05884aff507d71fca71a",
       "alias": "bolt.observer",
       "color": "#ffffff",
       "addresses": [
        ...
       ],
       "features": {
         "0": {
           "name": "data-loss-protect",
           "is_required": true,
           "is_known": true
         },
         ...
       },
       "last_update": 1668395085
     },
    "num_channels": 1337,
    "total_capacity": 73311337,
    "channels": [
    {
      "channel_id": 754581636042522625,
       "chan_point": "5ba4d1507db92c66772681f00b004205b2a2dcb63e2cbe7833f565e9a69212c7:1",
       "node1_pub": "0260fab633066ed7b1d9b9b8a0fac87e1579d1709e874d28a0d171a1f5c43bb877",
       "node2_pub": "0288037d3f0bdcfb240402b43b80cdc32e41528b3e2ebe05884aff507d71fca71a",
       "capacity": 13371337,
       "node1_policy": {
         "time_lock_delta": 40,
         "min_htlc": 1000,
         "fee_base_msat": 1000,
         "fee_rate_milli_msat": 1,
         "last_update": 1668339285,
         "max_htlc_msat": 500000000
       },

        ...
      ]

}
```

Actual channel balance is reported through `changed_channels` and if any channels closings were detected that is sent via `closed_channels`:

```
{
  "poll_interval": "10s",
  "allowed_entropy": 64,
  "allow_private_channels": false,
  "chain": "bitcoin",
  "network": "mainnet",
  "pubkey": "0288037d3f0bdcfb240402b43b80cdc32e41528b3e2ebe05884aff507d71fca71a",
  "timestamp": 1668425373,
  "changed_channels": [
    {
      "active": true,
      "private": false,
      "active_previous": true,
      "local_pubkey": "0288037d3f0bdcfb240402b43b80cdc32e41528b3e2ebe05884aff507d71fca71a",
      "remote_pubkey": "0260fab633066ed7b1d9b9b8a0fac87e1579d1709e874d28a0d171a1f5c43bb877",
      "chan_id": 754581636042522625,
      "capacity": 13371337,
      "remote_nominator": 10000000,
      "local_nominator": 3371337,
      "denominator": 13371337,
      "remote_nominator_diff": 0,
      "local_nominator_diff": 0,
      "denominator_diff": 0,
      "active_remote": true,
      "active_remote_previous": true,
      "active_local": true,
      "active_local_previous": true,
    },
    ...
   ],
  "closed_channels": [
    { "channel_id": 754581636042522626, "close_info": { "opener": "local", "closer": "remote", "close_type": "cooperative" } }
  ]
  }
```

`poll_interval` is the setting how often to poll for changes and `allowed_entropy` is the entropy in bits that you can also specify via `--allowedentropy` flag.
`local_nominator / denominator` is the fraction of how much balance is on your side and `remote_nominator / denominator` is on the other side of the channel.
If you use a high enough entropy `denominator` is same as `capacity` in satoshis and thus the `nominator` values can then be interpreted as balance on each side.
Note that `local + remote balance` is not necessary equal to `capacity` due to channel reserve and other factors.
