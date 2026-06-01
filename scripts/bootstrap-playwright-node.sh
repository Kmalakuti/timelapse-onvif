#!/usr/bin/env bash
set -euo pipefail

version="1.42.0"
root="tools/playwright/node_modules"

fetch_package() {
  local name="$1"
  local target="$root/$name"
  local archive="/tmp/${name}-${version}.tgz"
  if [ -f "$target/package.json" ]; then
    return
  fi
  mkdir -p "$target"
  curl -fsSL "https://registry.npmjs.org/${name}/-/${name}-${version}.tgz" -o "$archive"
  tar -xzf "$archive" --strip-components=1 -C "$target"
  rm -f "$archive"
}

fetch_package playwright-core
fetch_package playwright
