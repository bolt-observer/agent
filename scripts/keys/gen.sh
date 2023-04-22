#! /usr/bin/env nix-shell
#! nix-shell -i bash -p curl jq dos2unix

DEVS="dcrystalj fiksn"

rm -rf developers.asc

for dev in $DEVS; do
  curl https://api.github.com/users/${dev}/gpg_keys | jq -r '.[] | select(.emails[0].email | test(".*@bolt.observer")) | .raw_key' | dos2unix > ${dev}.asc
  cat ${dev}.asc >> developers.asc
done
