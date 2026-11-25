#!/usr/bin/env bash
# dev.sh builds Coddy and its plugins from this folder, installs the plugins, and opens
# Coddy in the foreground, its log in this terminal. Arguments go to Coddy:
#
#   ./dev.sh                     the folders open last time
#   ./dev.sh ~/Code/some-project
#
# When go-renderer and go-ui sit beside this folder, they are built from there, so a change in
# them shows at once.

set -euo pipefail

root="$(cd "$(dirname "$0")" && pwd)"
code_folder="$(dirname "$root")"

if [ -z "${GOWORK:-}" ] && [ -d "$code_folder/go-renderer" ] && [ -d "$code_folder/go-ui" ]; then
    GOWORK="$(mktemp --suffix=.work 2> /dev/null || mktemp -t coddy-dev)"
    export GOWORK
    trap 'rm -f "$GOWORK"' EXIT
    cat > "$GOWORK" << WORK
go 1.25.6
use $root/editor
use $root/sdk
use $root/plugins
replace github.com/akrck02/go-renderer => $code_folder/go-renderer
replace github.com/akrck02/go-ui => $code_folder/go-ui
WORK
fi

echo "Building Coddy…"
go build -C "$root/editor" -o "$root/out/coddy" ./cmd/coddy
echo "Building and installing the plugins…"
"$root/plugins/install.sh"
echo "Opening Coddy…"
"$root/out/coddy" "$@"
