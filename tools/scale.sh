#!/bin/bash
# 3.1 scale: N concurrent headless actors, per-instance RSS via time -v.
# Usage: scale.sh N OUTDIR
N=$1
OUT=$2
BIN=/tmp/fabric-spike/build/fabric-probe3
BUNDLE=/tmp/gate0-app/dist2/_expo/static/js/android/App-5fea9f309f26ca140031cb81833610e5.js
mkdir -p "$OUT"
export LD_LIBRARY_PATH=/tmp/deMainps/root/usr/lib/aarch64-linux-gnu:/tmp/hermes-014/build/lib
export HATCH_JSTIMERS_ID=184 HATCH_PUMP_CAP=200 HATCH_LINGER=25
START=$(date +%s%3N)
for i in $(seq 1 "$N"); do
  /usr/bin/time -v timeout 200 "$BIN" "$BUNDLE" > "$OUT/actor-$i.txt" 2>&1 &
done
wait
END=$(date +%s%3N)
echo "batch_n=$N batch_wall_ms=$((END - START))"
fails=0
for i in $(seq 1 "$N"); do
  grep -q "GATE0_TRANSACTIONS=" "$OUT/actor-$i.txt" || fails=$((fails + 1))
done
echo "actors_without_mount=$fails"
