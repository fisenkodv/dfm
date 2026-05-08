#!/usr/bin/env sh
# install.sh — fetch the latest dfm release and drop the binary into
# $DFM_INSTALL_DIR (default: ~/.local/bin).
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/fisenkodv/dfm/master/scripts/install.sh | sh
#   DFM_VERSION=v0.2.0 curl -fsSL ... | sh   # pin a specific version
#   DFM_INSTALL_DIR=/usr/local/bin sudo sh install.sh

set -eu

REPO="${DFM_REPO:-bitcldr/dfm}"
INSTALL_DIR="${DFM_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${DFM_VERSION:-latest}"

log()  { printf '==> %s\n' "$*" >&2; }
die()  { printf 'error: %s\n' "$*" >&2; exit 1; }

have() { command -v "$1" >/dev/null 2>&1; }

detect_platform() {
    uname_s=$(uname -s)
    uname_m=$(uname -m)
    case "$uname_s" in
        Darwin) os=macos ;;
        Linux)  os=linux ;;
        *) die "unsupported OS: $uname_s" ;;
    esac
    case "$uname_m" in
        x86_64|amd64)   arch=x86_64 ;;
        arm64|aarch64)  arch=arm64 ;;
        *) die "unsupported arch: $uname_m" ;;
    esac
    printf '%s_%s' "$os" "$arch"
}

resolve_version() {
    if [ "$VERSION" != "latest" ]; then
        printf '%s' "$VERSION"
        return
    fi
    # GitHub's redirect from /releases/latest reveals the tag in the Location header.
    if have curl; then
        curl -fsSLI -o /dev/null -w '%{url_effective}' \
            "https://github.com/$REPO/releases/latest" \
            | sed 's|.*/tag/||'
    else
        die "curl is required"
    fi
}

main() {
    have curl || die "curl is required"
    have tar  || die "tar is required"

    platform=$(detect_platform)
    version=$(resolve_version)
    [ -n "$version" ] || die "could not resolve latest version"

    archive="dfm_${version}_${platform}.tar.gz"
    url="https://github.com/$REPO/releases/download/${version}/${archive}"

    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT

    log "downloading $url"
    curl -fsSL "$url" -o "$tmp/$archive" || die "download failed"

    log "extracting"
    tar -xzf "$tmp/$archive" -C "$tmp"

    mkdir -p "$INSTALL_DIR"
    install -m 0755 "$tmp/dfm" "$INSTALL_DIR/dfm"

    log "installed $INSTALL_DIR/dfm ($version)"
    case ":$PATH:" in
        *":$INSTALL_DIR:"*) ;;
        *) log "note: $INSTALL_DIR is not on PATH" ;;
    esac
}

main "$@"
