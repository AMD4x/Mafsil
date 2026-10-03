#!/bin/sh
# Linux terminal-first installer. No sudo, shell profiles, service or PATH edits.
set -eu
action=install
version=v0.1.0
destination=${HOME:?HOME is required}/.local/share/mafsil
bundle=
expected=
fail() { printf '%s\n' "mafsil: $*" >&2; exit 1; }
while [ "$#" -gt 0 ]; do
    case "$1" in
        --action|--version|--destination|--bundle|--sha256)
            [ "$#" -ge 2 ] || fail "Missing value for $1"
            option=$1; value=$2; shift 2
            case "$option" in
                --action) action=$value;; --version) version=$value;;
                --destination) destination=$value;; --bundle) bundle=$value;; --sha256) expected=$value;;
            esac;;
        --help) printf '%s\n' 'install.sh [--action install|uninstall|status] [--version v0.1.0] [--destination DIR] [--bundle DIR] [--sha256 DIGEST]'; exit 0;;
        *) fail "Unknown argument: $1";;
    esac
done
case "$action" in install|uninstall|status) :;; *) fail 'Invalid action';; esac
[ "$(uname -s)" = Linux ] || fail 'This installer requires Linux.'
case "$(uname -m)" in x86_64|amd64) arch=amd64;; aarch64|arm64) arch=arm64;; *) fail 'Supported architectures: amd64, arm64';; esac
printf '%s\n' "$version" | LC_ALL=C grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || fail 'Use an explicit vMAJOR.MINOR.PATCH version.'
case "$destination" in /*) :;; *) fail 'Destination must be absolute.';; esac
assert_path() {
    inspect=$1
    while [ "$inspect" != / ] && [ "$inspect" != . ]; do
        [ ! -L "$inspect" ] || fail "Refusing symbolic-link path: $inspect"
        inspect=$(dirname -- "$inspect")
    done
}
assert_path "$destination"
destination=$(realpath -m -- "$destination")
[ "$destination" != / ] || fail 'A filesystem root cannot be an installation directory.'
parent=$(dirname -- "$destination")
marker=mafsil.install
managed='mafsil LICENSE THIRD_PARTY_NOTICES.md install.sh'
owned='mafsil LICENSE THIRD_PARTY_NOTICES.md install.sh mafsil.install'
has_record=0
read_record() {
    has_record=0
    [ -f "$destination/$marker" ] || return 0
    assert_path "$destination/$marker"
    [ "$(wc -c < "$destination/$marker")" -le 16384 ] || fail 'Installation record is too large.'
    [ "$(sed -n '1p' "$destination/$marker")" = 'product=Mafsil' ] || fail 'Unrecognized installation.'
    [ "$(sed -n '2p' "$destination/$marker")" = 'schema=1' ] || fail 'Unrecognized installation schema.'
    sed -n '3p' "$destination/$marker" | LC_ALL=C grep -Eq '^version=v[0-9]+\.[0-9]+\.[0-9]+$' || fail 'Invalid recorded version.'
    [ "$(wc -l < "$destination/$marker")" -eq 7 ] || fail 'Invalid installation file list.'
    for name in $managed; do
        assert_path "$destination/$name"
        [ -f "$destination/$name" ] || fail "Managed file is missing: $name"
        [ "$(stat -c %h -- "$destination/$name")" -eq 1 ] || fail "Hard-linked managed file: $name"
        digest=$(sha256sum -- "$destination/$name"); digest=${digest%% *}
        [ "$(LC_ALL=C grep -Fxc "$digest  $name" "$destination/$marker")" -eq 1 ] || fail "Managed file modified: $name. Preserve it and repair explicitly."
    done
    has_record=1
}
read_record
if [ "$action" = status ]; then
    printf 'installed=%s\ndestination=%s\n' "$has_record" "$destination"
    if [ "$has_record" -eq 1 ]; then sed -n '3p' "$destination/$marker"; fi
    exit 0
fi
mkdir -p -- "$parent"
assert_path "$parent"
lock=$destination.install-lock
mkdir -- "$lock" 2>/dev/null || fail "Another installer may be running, or a stale lock remains: $lock. Inspect it before manual removal."
stage=
backup=
saved=
changed=
committed=0
rollback_failed=0
cleanup() {
    rc=$?
    trap - EXIT HUP INT TERM
    set +e
    if [ "$committed" -eq 0 ]; then
        for name in $changed; do
            rm -f -- "$destination/$name" || rollback_failed=1
        done
        for name in $saved; do
            if [ -e "$destination/$name" ] || [ -L "$destination/$name" ]; then
                rollback_failed=1
            else
                mv -T -- "$backup/$name" "$destination/$name" || rollback_failed=1
            fi
        done
    fi
    if [ -n "$stage" ] && [ -d "$stage" ]; then
        if [ "$committed" -eq 1 ]; then
            for name in $owned; do rm -f -- "$backup/$name" || rc=1; done
        fi
        for name in $owned SHA256SUMS "mafsil_${version}_linux_${arch}"; do rm -f -- "$stage/$name" || rc=1; done
        rmdir -- "$backup" 2>/dev/null
        rmdir -- "$stage" 2>/dev/null
    fi
    rmdir -- "$lock" || rc=1
    if [ "$action" = uninstall ] && [ "$committed" -eq 1 ]; then rmdir -- "$destination" 2>/dev/null; fi
    if [ "$rollback_failed" -ne 0 ]; then
        printf 'Rollback incomplete. Retain %s for recovery.\n' "$backup" >&2
        rc=1
    fi
    exit "$rc"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' HUP TERM
read_record
if [ "$action" = uninstall ] && [ "$has_record" -eq 0 ]; then printf '%s\n' 'No managed Mafsil installation found.'; exit 0; fi
if [ "$has_record" -eq 0 ]; then
    for name in $owned; do [ ! -e "$destination/$name" ] && [ ! -L "$destination/$name" ] || fail "Refusing unowned file: $name"; done
fi
umask 077
stage=$(mktemp -d "$parent/.mafsil-install.XXXXXXXX")
backup=$stage/previous
mkdir -- "$backup"
if [ "$action" = install ]; then
    asset=mafsil_${version}_linux_${arch}
    for name in "$asset" SHA256SUMS LICENSE THIRD_PARTY_NOTICES.md install.sh; do
        if [ -n "$bundle" ]; then
            assert_path "$bundle/$name"
            cp -- "$bundle/$name" "$stage/$name"
        else
            curl --proto '=https' --proto-redir '=https' -fL --connect-timeout 15 --max-time 120 --max-filesize 134217728 \
                "https://github.com/AMD4x/Mafsil/releases/download/$version/$name" -o "$stage/$name"
        fi
    done
    [ "$(wc -c < "$stage/SHA256SUMS")" -le 65536 ] || fail 'Checksum manifest is too large.'
    LC_ALL=C awk 'NF != 2 || length($1) != 64 || $1 ~ /[^0-9a-fA-F]/ || $2 ~ /[^A-Za-z0-9_.-]/ || seen[$2]++ { exit 1 }' "$stage/SHA256SUMS" || fail 'Malformed or duplicate checksum manifest entry.'
    for name in "$asset" LICENSE THIRD_PARTY_NOTICES.md install.sh; do
        digest=$(sha256sum -- "$stage/$name"); digest=${digest%% *}
        [ "$(LC_ALL=C grep -Fxc "$digest  $name" "$stage/SHA256SUMS")" -eq 1 ] || fail "Checksum mismatch: $name"
        if [ "$name" = "$asset" ] && [ -n "$expected" ]; then [ "$expected" = "$digest" ] || fail 'Binary digest differs from --sha256.'; fi
    done
    mv -- "$stage/$asset" "$stage/mafsil"
    chmod 0755 "$stage/mafsil" "$stage/install.sh"
    chmod 0644 "$stage/LICENSE" "$stage/THIRD_PARTY_NOTICES.md"
    banner=$(timeout 10 "$stage/mafsil" version) || fail 'Candidate failed version check.'
    case "$banner" in "Mafsil ${version#v} ("*) :;; *) fail 'Candidate version differs from requested release.';; esac
    {
        printf 'product=Mafsil\nschema=1\nversion=%s\n' "$version"
        for name in $managed; do digest=$(sha256sum -- "$stage/$name"); digest=${digest%% *}; printf '%s  %s\n' "$digest" "$name"; done
    } > "$stage/$marker"
fi
mkdir -p -- "$destination"
assert_path "$destination"
move_new() {
    [ ! -e "$2" ] && [ ! -L "$2" ] || fail "Destination appeared during installation: $2"
    mv -T -n -- "$1" "$2"
    [ ! -e "$1" ] || fail "Could not publish $2"
}
for name in $owned; do
    if [ -f "$destination/$name" ]; then move_new "$destination/$name" "$backup/$name"; saved="$saved $name"; fi
done
if [ "$action" = install ]; then
    for name in $owned; do move_new "$stage/$name" "$destination/$name"; changed="$changed $name"; done
    read_record
fi
committed=1
if [ "$action" = install ]; then
    printf 'Installed Mafsil %s at %s\n' "$version" "$destination"
    printf 'Run: "%s/mafsil" help\n' "$destination"
    printf '%s\n' 'No PATH, startup or service settings were changed.'
else
    printf '%s\n' 'Removed managed Mafsil files. User configurations and unrelated files were preserved.'
fi
