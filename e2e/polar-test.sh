#!/usr/bin/env bash

NAME=${NAME:-polar-n1-alice}
# Those two come from outside
API_KEY=${API_KEY:-wrong}
SERVER=${SERVER:-"bolt.observer"}

TLS_BASE64=$(docker exec $NAME /bin/sh -c "cat /home/lnd/.lnd/tls.cert | base64 | tr -d '\n'")
RO_MACAROON_BASE64=$(docker exec $NAME /bin/sh -c "cat /home/lnd/.lnd/data/chain/bitcoin/regtest/readonly.macaroon | base64 | tr -d '\n'")

DIR=$(mktemp -d)
clean() {
  rm -rf $DIR
}

trap clean EXIT

echo $RO_MACAROON_BASE64 | base64 -d > $DIR/readonly.macaroon
echo $TLS_BASE64 | base64 -d > $DIR/tls.cert

docker run --network host -it -v $DIR:/tmp ghcr.io/bolt-observer/agent:latest --apikey $API_KEY --macaroonpath /tmp/readonly.macaroon --tlscertpath /tmp/tls.cert --rpcserver 127.0.0.1:10001 --interval manual --url "https://$SERVER/api/agent-report" --nodeurl "https://$SERVER/api/private-node" --verbosity 3 > $DIR/output.txt

cat $DIR/output.txt

grep -q "Sent out nodeinfo callback" $DIR/output.txt || { echo "Failed to send nodeinfo callback"; exit 1; }
grep -q "Sent out callback" $DIR/output.txt || { echo "Failed to send callback"; exit 1; }

rm -rf $DIR
echo "Success"
exit 0
