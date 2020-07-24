#!/bin/bash

set -euo pipefail

result=1
trap 'kill $(jobs -p); exit $result' EXIT

# shellcheck disable=SC1091
source .bingo/variables.env

#    --log.level=debug \
TARGETS=""
for i in $(seq 0 2); do
  (
    backend \
      --web.listen=0.0.0.0:808${i} \
      --web.internal.listen=0.0.0.0:818${i}
  ) &
  TARGETS="${TARGETS},http://127.0.0.1:808${i}/receive"
done

(
  proxy \
    --web.listen=0.0.0.0:8090 \
    --web.internal.listen=0.0.0.0:8091 \
    --web.targets="$TARGETS"
) &

echo "-------------------------------------------"
echo "- Waiting for dependencies to come up...  -"
echo "-------------------------------------------"
sleep 10

until curl --output /dev/null --silent --fail http://127.0.0.1:8091/-/ready; do
  printf '.'
  sleep 1
done

# Start three Prometheus servers monitoring themselves and remote write metrics.
for i in $(seq 0 2); do
  rm -rf data/source_prom${i}
  mkdir -p data/source_prom${i}/

  cat >data/source_prom${i}/prometheus.yml <<-EOF
global:
  external_labels:
    prometheus: prom-${i}
rule_files:
  - 'rules.yml'
scrape_configs:
- job_name: prometheus
  scrape_interval: 5s
  static_configs:
  - targets:
    - "127.0.0.1:5909${i}"
remote_write:
- url: http://127.0.0.1:8090/receive
EOF

  (
    ${PROMETHEUS} \
      --config.file data/source_prom${i}/prometheus.yml \
      --storage.tsdb.path data/source_prom${i} \
      --log.level warn \
      --web.enable-lifecycle \
      --storage.tsdb.min-block-duration=2h \
      --storage.tsdb.max-block-duration=2h \
      --web.listen-address 0.0.0.0:5909${i}
  ) &
  sleep 0.25
done

# Start a Prometheus server to scrape all others.
rm -rf data/prom/
mkdir -p data/prom/

cat >data/prom/prometheus.yml <<-EOF
global:
  external_labels:
    prometheus: prom-${i}
rule_files:
  - 'rules.yml'
scrape_configs:
- job_name: backend
  scrape_interval: 5s
  static_configs:
  - targets:
    - "127.0.0.1:8180"
    - "127.0.0.1:8181"
    - "127.0.0.1:8182"
- job_name: proxy
  scrape_interval: 5s
  static_configs:
  - targets:
    - "127.0.0.1:8091"
EOF

(
  ${PROMETHEUS} \
    --config.file data/prom/prometheus.yml \
    --storage.tsdb.path data/prom \
    --log.level warn \
    --web.enable-lifecycle \
    --storage.tsdb.min-block-duration=2h \
    --storage.tsdb.max-block-duration=2h \
    --web.listen-address 0.0.0.0:9091
) &

wait
