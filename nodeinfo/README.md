# Nodeinfo

Nodeinfo is an abstraction to gather data similar to `lncli getnodeinfo` periodically from nodes. Agent uses that only if you allow access to private data (`--private` flag). We need it to have a consistent state of the node because it could have unannounced channels.Without this we fallback to public data from the lightning network gossip.
