#!/usr/bin/env bash
# install.sh builds Coddy's plugins, official and examples, and installs them in Coddy's plugins
# folder. A plugin installed is on; Settings › Plugins turns it off.
#
# Usage:
#   plugins/install.sh               every plugin
#   plugins/install.sh go github     only these
#
# A plugin is a folder with a plugin.json; when it also holds Go code, its program is built beside
# the plugin.json, named as the folder (the name its "command" runs). Its SVG and PNG pictures,
# which its "icon" may name, are copied too.

set -euo pipefail

plugins_source="$(cd "$(dirname "$0")" && pwd)"

# installed_folder is Coddy's plugins folder: in the user's configuration folder, as Go's
# os.UserConfigDir finds it.
installed_folder() {
    if [ "$(uname)" = "Darwin" ]; then
        echo "$HOME/Library/Application Support/coddy/plugins"
        return
    fi
    echo "${XDG_CONFIG_HOME:-$HOME/.config}/coddy/plugins"
}

# TODO(akrck02): drop this workspace once go-ui is released with its remote package; the plugins
# then build alone, against the versions their go.mod pins.
# use_neighbours builds with the go-renderer and go-ui beside Coddy, when they are there and no
# workspace was given: Coddy's sdk, which the plugins build on, uses changes of them not released
# yet.
use_neighbours() {
    local code_folder
    code_folder="$(dirname "$(dirname "$plugins_source")")"
    if [ -n "${GOWORK:-}" ] || [ ! -d "$code_folder/go-ui" ] || [ ! -d "$code_folder/go-renderer" ]; then
        return
    fi
    GOWORK="$(mktemp --suffix=.work 2> /dev/null || mktemp -t coddy-plugins)"
    export GOWORK
    trap 'rm -f "$GOWORK"' EXIT
    cat > "$GOWORK" << WORK
go 1.25.6
use $(dirname "$plugins_source")/sdk
use $plugins_source
replace github.com/akrck02/go-renderer => $code_folder/go-renderer
replace github.com/akrck02/go-ui => $code_folder/go-ui
WORK
}

install_plugin() {
    local name="$1" destination="$2"
    local source="$plugins_source/$name"
    [ -f "$source/plugin.json" ] || {
        echo "install: $name is not a plugin (no $source/plugin.json)" >&2
        return 1
    }
    mkdir -p "$destination/$name"
    cp "$source/plugin.json" "$destination/$name/"
    # the pictures a plugin.json may name as its icon
    for picture in "$source"/*.svg "$source"/*.png; do
        if [ -f "$picture" ]; then
            cp "$picture" "$destination/$name/"
        fi
    done
    if compgen -G "$source/*.go" > /dev/null; then
        go build -C "$plugins_source" -o "$destination/$name/$name" "./$name"
    fi
    echo "Installed $name"
}

destination="$(installed_folder)"
use_neighbours
if [ "$#" -eq 0 ]; then
    for folder in "$plugins_source"/*/; do
        set -- "$@" "$(basename "$folder")"
    done
fi
for name in "$@"; do
    install_plugin "$name" "$destination"
done
echo "In $destination. Coddy's Settings › Plugins turns them off."
