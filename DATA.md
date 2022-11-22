# What kind of data does the agent report?

You can always check by running it with following flags, which forces a dry-run (without sending anything to the server)

```
--url "" -nodeurl "" --verbosity 2
```

basically it will report a JSON similar to https://api.lightning.community/#getnodeinfo every 10 minutes except that if you allow private data to be reported (`-private`) also unnanounced channels are included. The only additional
field is `timestamp` in UNIX time. We use that data so the information about a node is available (in case gossip data has not arrived yet or won't arrive when node is private).

```
{
  "timestamp": 1668425373,
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

and then upon every change (configurable poll interval) it will report something like:

```
{
  "poll_interval": "10s",
  "allowed_entropy": 64,
  "allow_private_channels": false,
  "chain": "bitcoin",
  "network": "mainnet",
  "pubkey": "",
  "timestamp": 1668425373,
  "changed_channels": [
    {
      "active": false,
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
      "nominator": 13371337,
      "nominator_diff": 0
    },
    ...
   ],
  "closed_channels": [
    { "channel_id": 754581636042522626 }
  ]
  }
```

`poll_interval` is the setting how often to poll for changes and `allowed_entropy` is the entropy in bits that you can also specify via `-allowedentropy` flag.
`local_nominator / denominator` is the fraction of how much balance is on your side and `remote_nominator / denominator` is on the other side of the channel.
If you use a high enough entropy `denominator` is same as `capacity` in satoshis and thus the `nominator` values can then be interpreted as satoshis.
note that `local + remote balance` is not necessary equal to `capacity` due to channel reserve and other factors.

`nominator` and `nominator_diff` are deprecated and will be removed soon.
