#!/bin/bash

set -euo pipefail

result=1
trap 'kill $(jobs -p); exit $result' EXIT

# shellcheck disable=SC1091
source .bingo/variables.env

(
  backend \
    --web.listen=0.0.0.0:8080 \
    --web.internal.listen=0.0.0.0:8081 \
    --log.level=debug
) &

(
  proxy \
    --web.listen=0.0.0.0:8090 \
    --web.internal.listen=0.0.0.0:8091 \
    --web.targets=http://127.0.0.1:8080/receive \
    --log.level=debug
) &

echo "-------------------------------------------"
echo "- Waiting for dependencies to come up...  -"
echo "-------------------------------------------"
sleep 10

until curl --output /dev/null --silent --fail http://127.0.0.1:8091/-/ready; do
  printf '.'
  sleep 1
done

echo "-------------------------------------------"
echo "- Metrics Remote Write tests                           -"
echo "-------------------------------------------"

if $UP \
  --listen=0.0.0.0:8888 \
  --endpoint-type=metrics \
  --endpoint-write=http://0.0.0.0:8090/receive \
  --period=500ms \
  --initial-query-delay=250ms \
  --threshold=1 \
  --latency=10s \
  --duration=10s \
  --log.level=error \
  --name=remote_write \
  --labels='_id="test"'; then
  result=0
  echo "-------------------------------------------"
  echo "- Metrics Remote Write tests: OK                        -"
  echo "-------------------------------------------"
else
  result=1
  echo "-------------------------------------------"
  echo "- Metrics Remote Write tests: FAILED                   -"
  echo "-------------------------------------------"
  exit 1
fi

echo "-------------------------------------------"
echo "- All tests: OK                           -"
echo "-------------------------------------------"
exit 0
