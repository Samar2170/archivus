#!/usr/bin/env bash
#
# Archivus installer.
#
# Run from an unpacked release tarball:
#   tar -xzf archivus-<version>-linux-amd64.tar.gz
#   cd archivus-<version>-linux-amd64
#   ./install.sh
#
# Installs to /usr/local when run as root, otherwise to ~/.local.

set -euo pipefail

PROJECT_NAME=archivus
SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

MODE=home
SYSTEMD=0
UNINSTALL=0

if [ "$(id -u)" -eq 0 ]; then
	PREFIX=/usr/local
else
	PREFIX="$HOME/.local"
fi

usage() {
	cat <<EOF
Usage: ./install.sh [options]

Options:
  --prefix DIR   Install prefix (default: $PREFIX)
  --mode MODE    Server mode for the systemd unit: home or biz (default: home)
  --systemd      Install and enable a systemd service
  --uninstall    Remove a previous installation
  -h, --help     Show this help

Installs:
  PREFIX/lib/$PROJECT_NAME/    binary and frontend assets
  PREFIX/bin/$PROJECT_NAME     symlink onto PATH

Config and data are created by $PROJECT_NAME on first run under
~/.archivus and are never written or overwritten by this script.
EOF
}

while [ $# -gt 0 ]; do
	case "$1" in
	--prefix)
		PREFIX="${2:?--prefix needs a directory}"
		shift 2
		;;
	--mode)
		MODE="${2:?--mode needs home or biz}"
		shift 2
		;;
	--systemd)
		SYSTEMD=1
		shift
		;;
	--uninstall)
		UNINSTALL=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "unknown option: $1" >&2
		usage >&2
		exit 1
		;;
	esac
done

LIB_DIR="$PREFIX/lib/$PROJECT_NAME"
BIN_LINK="$PREFIX/bin/$PROJECT_NAME"

if [ "$(id -u)" -eq 0 ]; then
	SERVICE_FILE=/etc/systemd/system/$PROJECT_NAME.service
	SYSTEMCTL="systemctl"
else
	SERVICE_FILE="$HOME/.config/systemd/user/$PROJECT_NAME.service"
	SYSTEMCTL="systemctl --user"
fi

if [ "$UNINSTALL" -eq 1 ]; then
	echo "==> Uninstalling $PROJECT_NAME"
	if [ -f "$SERVICE_FILE" ]; then
		$SYSTEMCTL disable --now "$PROJECT_NAME.service" 2>/dev/null || true
		rm -f "$SERVICE_FILE"
		$SYSTEMCTL daemon-reload 2>/dev/null || true
	fi
	rm -rf "$LIB_DIR"
	rm -f "$BIN_LINK"
	echo "==> Removed. Your data in ~/.archivus was left untouched."
	exit 0
fi

case "$MODE" in
home | biz) ;;
*)
	echo "error: --mode must be 'home' or 'biz', got '$MODE'" >&2
	exit 1
	;;
esac

if [ ! -x "$SRC_DIR/$PROJECT_NAME" ]; then
	echo "error: $SRC_DIR/$PROJECT_NAME not found or not executable." >&2
	echo "Run this script from an unpacked release tarball." >&2
	exit 1
fi

if [ ! -f "$SRC_DIR/static/index.html" ]; then
	echo "error: $SRC_DIR/static/index.html not found; the release looks incomplete." >&2
	exit 1
fi

echo "==> Installing $PROJECT_NAME to $PREFIX"
mkdir -p "$LIB_DIR" "$PREFIX/bin"
install -m 0755 "$SRC_DIR/$PROJECT_NAME" "$LIB_DIR/$PROJECT_NAME"
rm -rf "$LIB_DIR/static"
cp -r "$SRC_DIR/static" "$LIB_DIR/static"
ln -sfn "$LIB_DIR/$PROJECT_NAME" "$BIN_LINK"

if [ "$SYSTEMD" -eq 1 ]; then
	if ! command -v systemctl >/dev/null 2>&1; then
		echo "error: --systemd given but systemctl is not available." >&2
		exit 1
	fi
	echo "==> Installing systemd service ($MODE mode)"
	mkdir -p "$(dirname "$SERVICE_FILE")"
	{
		echo "[Unit]"
		echo "Description=Archivus file archiver"
		echo "After=network.target"
		echo
		echo "[Service]"
		echo "ExecStart=$LIB_DIR/$PROJECT_NAME server -m $MODE"
		echo "Restart=on-failure"
		if [ "$(id -u)" -eq 0 ]; then
			echo "User=$SUDO_USER"
		fi
		echo
		echo "[Install]"
		if [ "$(id -u)" -eq 0 ]; then
			echo "WantedBy=multi-user.target"
		else
			echo "WantedBy=default.target"
		fi
	} >"$SERVICE_FILE"
	$SYSTEMCTL daemon-reload
	$SYSTEMCTL enable --now "$PROJECT_NAME.service"
	echo "==> Service started: $SYSTEMCTL status $PROJECT_NAME"
fi

echo
echo "==> Installed $PROJECT_NAME to $LIB_DIR"
case ":$PATH:" in
*":$PREFIX/bin:"*) ;;
*) echo "    Note: $PREFIX/bin is not on your PATH." ;;
esac

if [ "$MODE" = biz ] && [ ! -f "$HOME/s3_config.yaml" ]; then
	echo "    Note: biz mode reads S3 credentials from ~/s3_config.yaml, which does not exist yet."
fi

if [ "$SYSTEMD" -eq 0 ]; then
	echo
	echo "    Start it with:  $PROJECT_NAME server -m $MODE"
	echo "    Then open:      http://localhost:8080"
fi
