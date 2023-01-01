#!/usr/bin/env bash

set -e

if [ -z "${1}" ]; then
  bench=benchmarks
  tag=all
else
  bench=bench-"${1}"
  tag="${1}"
fi

before=$(git describe --abbrev=10 --always)."${tag}".before
if [ ! -f "${before}" ]; then
  rm -rf *."${tag}".before
  git stash push -m "after" >/dev/null 2>&1
  make -s "${bench}" >"${before}"
  git stash pop stash@{0} >/dev/null 2>&1
fi
make -s "${bench}" >stash.after

benchstat --geomean "${before}" stash.after
rm -rf stash.after
