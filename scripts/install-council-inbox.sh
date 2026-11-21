#!/bin/sh
# Install the council-inbox CLI from a GitHub release, no Go or clone needed.
#   curl -fsSL https://raw.githubusercontent.com/iksnerd/council-hub/main/scripts/install-council-inbox.sh | sh
# Env: COUNCIL_INBOX_VERSION  tag such as v0.68.0 (default: latest release)
#      COUNCIL_INBOX_DIR      install dir (default: ~/.local/bin)
#      COUNCIL_INBOX_BASE_URL override the download base (for testing)
set -eu

repo="iksnerd/council-hub"
dir="${COUNCIL_INBOX_DIR:-$HOME/.local/bin}"
version="${COUNCIL_INBOX_VERSION:-latest}"

die() { echo "install-council-inbox: $*" >&2; exit 1; }

case "$(uname -s)" in
	Darwin) os=darwin ;;
	Linux) os=linux ;;
	*) die "unsupported OS $(uname -s); build from source (docs/agent-inbox.md)" ;;
esac
case "$(uname -m)" in
	arm64|aarch64) arch=arm64 ;;
	x86_64|amd64) arch=amd64 ;;
	*) die "unsupported architecture $(uname -m); build from source (docs/agent-inbox.md)" ;;
esac

if [ -n "${COUNCIL_INBOX_BASE_URL:-}" ]; then
	base="$COUNCIL_INBOX_BASE_URL"
elif [ "$version" = latest ]; then
	base="https://github.com/$repo/releases/latest/download"
else
	base="https://github.com/$repo/releases/download/$version"
fi

asset="council-inbox_${os}_${arch}"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

curl -fsSL "$base/$asset" -o "$tmp/$asset" || die "download failed: $base/$asset"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" || die "download failed: $base/checksums.txt"

want=$(awk -v a="$asset" '$2 == a || $2 == "*" a { print $1 }' "$tmp/checksums.txt")
[ -n "$want" ] || die "no checksum for $asset in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
	got=$(sha256sum "$tmp/$asset" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
	got=$(shasum -a 256 "$tmp/$asset" | awk '{ print $1 }')
else
	die "need sha256sum or shasum to verify the download"
fi
[ "$got" = "$want" ] || die "checksum mismatch for $asset (wanted $want, got $got)"

mkdir -p "$dir"
chmod 755 "$tmp/$asset"
mv "$tmp/$asset" "$dir/council-inbox"
echo "Installed $dir/council-inbox"
case ":$PATH:" in
	*":$dir:"*) ;;
	*) echo "Note: $dir is not on your PATH." ;;
esac
