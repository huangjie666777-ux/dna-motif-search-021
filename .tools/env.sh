# Source from Bash to use this checkout's own Go toolchain and caches.
_pairwise_tool_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
export GOROOT="$_pairwise_tool_root/go"
export GOPATH="$_pairwise_tool_root/gopath"
export GOMODCACHE="$GOPATH/pkg/mod"
export GOCACHE="$_pairwise_tool_root/cache"
export GOTMPDIR="$_pairwise_tool_root/tmp"
export GOTOOLCHAIN=local
export GOENV=off
export GOTELEMETRY=off
export PATH="$GOROOT/bin:$PATH"
mkdir -p "$GOCACHE" "$GOTMPDIR" "$GOPATH"
unset _pairwise_tool_root
