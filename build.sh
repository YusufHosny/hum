#!/usr/bin/env bash
# Build hum for Linux and/or Windows.
#
# The app uses CGO (malgo for audio, opus.v2 for the Opus codec), so the
# Windows target is cross-compiled with mingw and statically linked against
# libogg/libopus/libopusfile that were built for x86_64-w64-mingw32.
#
# Usage:
#   ./build.sh            # build both linux + windows
#   ./build.sh linux      # linux only
#   ./build.sh windows    # windows only
#
# Override toolchain locations via env if your setup differs:
#   GO         path to the go binary            (default ~/.local/go/bin/go)
#   W64_PREFIX mingw install prefix for opus    (default ~/w64)
#   WIN_OUT    output path for the .exe         (default /mnt/c/Users/yusuf/hum/hum.exe)
set -euo pipefail

cd "$(dirname "$0")"

GO="${GO:-$HOME/.local/go/bin/go}"
W64_PREFIX="${W64_PREFIX:-$HOME/w64}"
WIN_OUT="${WIN_OUT:-/mnt/c/Users/yusuf/hum/hum.exe}"
target="${1:-both}"

build_linux() {
	echo "==> Linux  -> ./bin/hum"
	CGO_ENABLED=1 "$GO" build -o ./bin/hum ./cmd/hum
}

build_windows() {
	echo "==> Windows -> $WIN_OUT"
	mkdir -p "$(dirname "$WIN_OUT")"
	# --start-group is required: pkg-config emits the static libs in an order
	# that the linker can't resolve on its own (opusfile depends on opus+ogg).
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
		CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
		PKG_CONFIG_PATH="$W64_PREFIX/lib/pkgconfig" \
		PKG_CONFIG_LIBDIR="$W64_PREFIX/lib/pkgconfig" \
		CGO_LDFLAGS="-L$W64_PREFIX/lib -Wl,--start-group -lopusfile -lopus -logg -Wl,--end-group" \
		"$GO" build -ldflags "-extldflags=-static" -o "$WIN_OUT" ./cmd/hum
}

case "$target" in
	linux)   build_linux ;;
	windows) build_windows ;;
	both)    build_linux; build_windows ;;
	*) echo "usage: $0 [linux|windows|both]" >&2; exit 2 ;;
esac

echo "Done."
