#!/usr/bin/env bash

# Set outside usually
API_KEY=${API_KEY:-wrong}
SERVER=${SERVER:-"bolt.observer"}
DATASTORE_SERVER=${DATASTORE_SERVER:-"agent-api.bolt.observer:443"}
TAG=${TAG:-"unstable"}

NAME=${NAME:-polar-n3-alice}

DIR=$(mktemp -d)
chmod 777 $DIR
clean() {
  rm -rf $DIR
}

trap clean EXIT

echo "Scenario1 is running..."

# wait for data
runtime="1 minute"
endtime=$(date -ud "$runtime" +%s)

while [[ $(date -u +%s) -le $endtime ]]
do
  TLS_BASE64=$(docker exec $NAME /bin/sh -c "cat /home/lnd/.lnd/tls.cert | base64 | tr -d '\n'")
  RO_MACAROON_BASE64=$(docker exec $NAME /bin/sh -c "cat /home/lnd/.lnd/data/chain/bitcoin/regtest/readonly.macaroon | base64 | tr -d '\n'")

  echo $RO_MACAROON_BASE64 | base64 -d > $DIR/readonly.macaroon
  echo $TLS_BASE64 | base64 -d > $DIR/tls.cert

  SIZE1=$(wc -c <"$DIR/readonly.macaroon")
  SIZE2=$(wc -c <"$DIR/tls.cert")

  if [ "$SIZE1" -gt 16 ] && [ "$SIZE2" -gt 16 ]; then
    echo "Files prepared"
    break
  fi

  echo "Sleeping..."
  sleep 5
done

SIZE1=$(wc -c <"$DIR/readonly.macaroon")
SIZE2=$(wc -c <"$DIR/tls.cert")

if [ "$SIZE1" -le 16 ] || [ "$SIZE2" -le 16 ]; then
  echo "Files are missing"
  exit 1
fi

chmod 666 $DIR/*

# Ugly but lnd needs some time to become ready
sleep 30

echo "----"
docker run -t --network host -v $DIR:/tmp ghcr.io/bolt-observer/agent:$TAG --apikey $API_KEY --macaroonpath /tmp/readonly.macaroon --tlscertpath /tmp/tls.cert --rpcserver 127.0.0.1:10001 --interval manual --url "https://$SERVER/api/node-data-report/" --datastore-url $DATASTORE_SERVER --verbosity 3
echo "----"

# Normal run
docker run -t --network host -v $DIR:/tmp ghcr.io/bolt-observer/agent:$TAG --apikey $API_KEY --macaroonpath /tmp/readonly.macaroon --tlscertpath /tmp/tls.cert --rpcserver 127.0.0.1:10001 --interval manual --url "https://$SERVER/api/node-data-report/" --datastore-url $DATASTORE_SERVER --verbosity 3 2>&1 > $DIR/output.txt
cat $DIR/output.txt

grep -q "Sent out nodeinfo callback" $DIR/output.txt || { echo "Failed to send nodeinfo callback"; exit 1; }
grep -q "Sent out callback" $DIR/output.txt || { echo "Failed to send callback"; exit 1; }

NUM_CHANS=$(cat $DIR/output.txt | grep "Sent out nodeinfo callback" | sed -E 's/.* Sent out nodeinfo callback (.*)/\1/g' | jq '.num_channels' | tr -d " ")
if [ "$NUM_CHANS" != "2" ]; then
  echo "Number of channels wrong $NUM_CHANS - should be 2"
  exit 1
fi

# Private run
docker run -t --network host -v $DIR:/tmp ghcr.io/bolt-observer/agent:$TAG --apikey $API_KEY --macaroonpath /tmp/readonly.macaroon --tlscertpath /tmp/tls.cert --rpcserver 127.0.0.1:10001 --interval manual --url "https://$SERVER/api/node-data-report/" --datastore-url $DATASTORE_SERVER --verbosity 3 --private 2>&1 > $DIR/output.txt
cat $DIR/output.txt

grep -q "Sent out nodeinfo callback" $DIR/output.txt || { echo "Failed to send nodeinfo callback"; exit 1; }
grep -q "Sent out callback" $DIR/output.txt || { echo "Failed to send callback"; exit 1; }

NUM_CHANS=$(cat $DIR/output.txt | grep "Sent out nodeinfo callback" | sed -E 's/.* Sent out nodeinfo callback (.*)/\1/g' | jq '.num_channels' | tr -d " ")
if [ "$NUM_CHANS" != "3" ]; then
  echo "Number of channels wrong $NUM_CHANS - should be 3"
  exit 1
fi

rm -rf $DIR

echo "Scenario1 is running... SUCCESS"
exit 0
