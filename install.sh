#!/bin/sh
# Parse the complete bootstrap before running it when input arrives through a pipe.
mafsil_bootstrap() (
    set -eu
    version=latest
    destination=
    fail() { printf 'mafsil: %s\n' "$*" >&2; exit 1; }
    while [ "$#" -gt 0 ]; do
        case "$1" in
            --version|--destination)
                [ "$#" -ge 2 ] || fail "Missing value for $1"
                option=$1; value=$2; shift 2
                case "$option" in --version) version=$value;; --destination) destination=$value;; esac;;
            --help) printf '%s\n' 'Install or update Mafsil: install.sh [--version latest|vMAJOR.MINOR.PATCH] [--destination DIR]'; exit 0;;
            *) fail "Unknown argument: $1";;
        esac
    done
    [ "$(uname -s)" = Linux ] || fail 'This installer requires Linux.'
    case "$(uname -m)" in x86_64|amd64|aarch64|arm64) :;; *) fail 'Supported architectures: x64 and ARM64';; esac
    if [ "$version" = latest ]; then
        resolved=$(curl --proto '=https' --proto-redir '=https' -fsSIL --max-redirs 5 --connect-timeout 15 --max-time 30 \
            -o /dev/null -w '%{url_effective}' 'https://github.com/AMD4x/Mafsil/releases/latest') || fail 'Could not resolve the latest stable release.'
        case "$resolved" in
            https://github.com/AMD4x/Mafsil/releases/tag/*) version=${resolved#https://github.com/AMD4x/Mafsil/releases/tag/};;
            *) fail 'Latest release returned an unexpected destination.';;
        esac
    fi
    case "$version" in *[!v0-9.]*|'') fail 'Use latest or an explicit vMAJOR.MINOR.PATCH version.';; esac
    printf '%s\n' "$version" | LC_ALL=C grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || fail 'Invalid release version.'
    printf 'Installing or updating Mafsil %s\n' "$version"
    umask 077
    stage=$(mktemp -d "${TMPDIR:-/tmp}/mafsil-bootstrap.XXXXXXXX")
    cleanup() {
        rc=$?
        trap - EXIT HUP INT TERM
        rm -f -- "$stage/SHA256SUMS" "$stage/install.sh"
        rmdir -- "$stage"
        exit "$rc"
    }
    trap cleanup EXIT
    trap 'exit 130' INT
    trap 'exit 143' HUP TERM
    base=https://github.com/AMD4x/Mafsil/releases/download/$version
    curl --proto '=https' --proto-redir '=https' -fsSL --max-redirs 5 --connect-timeout 15 --max-time 60 --max-filesize 65536 \
        "$base/SHA256SUMS" -o "$stage/SHA256SUMS"
    curl --proto '=https' --proto-redir '=https' -fsSL --max-redirs 5 --connect-timeout 15 --max-time 60 --max-filesize 1048576 \
        "$base/install.sh" -o "$stage/install.sh"
    LC_ALL=C awk 'NF != 2 || length($1) != 64 || $1 ~ /[^0-9a-f]/ || $2 ~ /[^A-Za-z0-9_.-]/ || seen[$2]++ { exit 1 }' "$stage/SHA256SUMS" || fail 'Malformed or duplicate checksum entry.'
    (cd "$stage" && LC_ALL=C grep '  install.sh$' SHA256SUMS | sha256sum --strict -c -) || fail 'Release installer checksum mismatch.'
    if [ -n "$destination" ]; then
        sh "$stage/install.sh" --version "$version" --destination "$destination"
    else
        sh "$stage/install.sh" --version "$version"
    fi
)
mafsil_bootstrap "$@"
