#!/bin/bash

echo "checking for tools .........."
which pwgen || (echo missing pwgen; exit 1)
which wget || (echo missing wget; exit 1)
which cf || (echo missing cf; exit 1)

repo=https://github.wdf.sap.corp/cloudfoundry/chisel.git
name=chisel
appname=$(pwgen -N 1 -0)

if [[ $(basename $(pwd)) != $name ]]; then
  git clone $repo $name || (echo failed cloning chisel, exit 1)
  cd chisel
  echo echo '{ "foo:bar": [""] }' > auth.json
fi

cf push $appname || (cf delete $appname -f; exit 1)
domain=$(cf app $appname | grep urls | awk '{print $2}')
bin/chisel_linux_amd64 client --skip-ssl-validation --auth foo:bar \
    https://$domain 12345:www.google.com:80 &
pid=$!
if wget --tries 1 --read-timeout 2 -O /dev/null  https://$domain; then
    kill $pid
    cf delete $appname -f
    echo "chisel test OK"
    exit 0
else
    kill $pid
    cf delete $appname -f
    echo "chisel test failed"
    exit 1
fi


