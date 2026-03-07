#!/bin/bash
# export TS_AUTHKEY="tskey-xxx"
# export TS_LOGIN_SERVER="http://foo.bar.com"
# Only for reference
for c in cluster1:na cluster2:nb cluster3:nc; do
  IFS=: read -r ctx name <<< "$c"
  make ARGS="--authkey $TS_AUTHKEY --extra-args='--login-server $TS_LOGIN_SERVER' --context $ctx --cluster-name $name" uninstall
  make ARGS="--authkey $TS_AUTHKEY --extra-args='--login-server $TS_LOGIN_SERVER' --context $ctx --cluster-name $name" install
done
