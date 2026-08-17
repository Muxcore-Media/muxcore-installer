#!/usr/bin/env bash
# Fail if installer versions.env pins disagree with spool/tags/*.json for shared modules.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export MUXCORE_VERSIONS_ENV="${MUXCORE_VERSIONS_ENV:-$ROOT/versions.env}"
export MUXCORE_SPOOL_TAGS="${MUXCORE_SPOOL_TAGS:-$ROOT/../spool/tags}"

if [[ ! -f "$MUXCORE_VERSIONS_ENV" ]]; then
  echo "FAIL: missing $MUXCORE_VERSIONS_ENV" >&2
  exit 1
fi
if [[ ! -d "$MUXCORE_SPOOL_TAGS" ]]; then
  echo "FAIL: missing spool tags dir $MUXCORE_SPOOL_TAGS" >&2
  exit 1
fi

node <<'NODE'
const fs = require("fs");
const envPath = process.env.MUXCORE_VERSIONS_ENV;
const tagsDir = process.env.MUXCORE_SPOOL_TAGS;
const env = fs.readFileSync(envPath, "utf8");
const pins = {};
for (const line of env.split("\n")) {
  const m = line.match(/^([a-z0-9-]+)=(v[0-9][0-9.]*)$/);
  if (m) pins[m[1]] = m[2];
}
const core = env.match(/^CORE_TAG=(v[0-9.]+)/m);
if (core) pins.core = core[1];

let failed = 0;
for (const f of fs.readdirSync(tagsDir).filter((x) => x.endsWith(".json"))) {
  const d = JSON.parse(fs.readFileSync(`${tagsDir}/${f}`, "utf8"));
  for (const mod of d.modules || []) {
    const name = mod.repo.replace(/\/$/, "").split("/").pop();
    if (!pins[name]) continue;
    if (pins[name] !== mod.version) {
      console.error(`MISMATCH ${f}: ${name} spool=${mod.version} installer=${pins[name]}`);
      failed++;
    }
  }
}
if (failed) {
  console.error(`FAIL: ${failed} pin mismatch(es). Update versions.env and spool/tags together (see PIN-MATRIX.md).`);
  process.exit(1);
}
console.log("OK: installer pins match overlapping spool tag versions");
NODE
