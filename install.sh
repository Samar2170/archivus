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
  --mode MODE    Server mode for the systemd units: home or biz (default: home)
  --systemd      Install and enable systemd services (server + cron)
  --uninstall    Remove a previous installation
  -h, --help     Show this help

Installs:
  PREFIX/lib/$PROJECT_NAME/       binaries and frontend assets
  PREFIX/bin/$PROJECT_NAME        symlink onto PATH
  PREFIX/bin/$PROJECT_NAME-cron   symlink onto PATH (background job scheduler)

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

CRON_NAME="$PROJECT_NAME-cron"
LIB_DIR="$PREFIX/lib/$PROJECT_NAME"
BIN_LINK="$PREFIX/bin/$PROJECT_NAME"
CRON_BIN_LINK="$PREFIX/bin/$CRON_NAME"

if [ "$(id -u)" -eq 0 ]; then
	SERVICE_DIR=/etc/systemd/system
	SYSTEMCTL="systemctl"
else
	SERVICE_DIR="$HOME/.config/systemd/user"
	SYSTEMCTL="systemctl --user"
fi
SERVICE_FILE="$SERVICE_DIR/$PROJECT_NAME.service"
CRON_SERVICE_FILE="$SERVICE_DIR/$CRON_NAME.service"

if [ "$UNINSTALL" -eq 1 ]; then
	echo "==> Uninstalling $PROJECT_NAME"
	RELOAD=0
	for unit in "$CRON_NAME" "$PROJECT_NAME"; do
		if [ -f "$SERVICE_DIR/$unit.service" ]; then
			$SYSTEMCTL disable --now "$unit.service" 2>/dev/null || true
			rm -f "$SERVICE_DIR/$unit.service"
			RELOAD=1
		fi
	done
	[ "$RELOAD" -eq 1 ] && $SYSTEMCTL daemon-reload 2>/dev/null || true
	rm -rf "$LIB_DIR"
	rm -f "$BIN_LINK" "$CRON_BIN_LINK"
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

for bin in "$PROJECT_NAME" "$CRON_NAME"; do
	if [ ! -x "$SRC_DIR/$bin" ]; then
		echo "error: $SRC_DIR/$bin not found or not executable." >&2
		echo "Run this script from an unpacked release tarball." >&2
		exit 1
	fi
done

if [ ! -f "$SRC_DIR/static/index.html" ]; then
	echo "error: $SRC_DIR/static/index.html not found; the release looks incomplete." >&2
	exit 1
fi

echo "==> Installing $PROJECT_NAME to $PREFIX"
mkdir -p "$LIB_DIR" "$PREFIX/bin"
install -m 0755 "$SRC_DIR/$PROJECT_NAME" "$LIB_DIR/$PROJECT_NAME"
install -m 0755 "$SRC_DIR/$CRON_NAME" "$LIB_DIR/$CRON_NAME"
rm -rf "$LIB_DIR/static"
cp -r "$SRC_DIR/static" "$LIB_DIR/static"
ln -sfn "$LIB_DIR/$PROJECT_NAME" "$BIN_LINK"
ln -sfn "$LIB_DIR/$CRON_NAME" "$CRON_BIN_LINK"

if [ "$SYSTEMD" -eq 1 ]; then
	if ! command -v systemctl >/dev/null 2>&1; then
		echo "error: --systemd given but systemctl is not available." >&2
		exit 1
	fi
	echo "==> Installing systemd services ($MODE mode)"
	mkdir -p "$SERVICE_DIR"
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

	# The cron scheduler shares the store with the server, so start it after
	# the server unit. It is not a hard dependency: it can run on its own.
	{
		echo "[Unit]"
		echo "Description=Archivus background job scheduler"
		echo "After=network.target $PROJECT_NAME.service"
		echo
		echo "[Service]"
		echo "ExecStart=$LIB_DIR/$CRON_NAME -m $MODE"
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
	} >"$CRON_SERVICE_FILE"

	$SYSTEMCTL daemon-reload
	$SYSTEMCTL enable --now "$PROJECT_NAME.service"
	$SYSTEMCTL enable --now "$CRON_NAME.service"
	echo "==> Services started: $SYSTEMCTL status $PROJECT_NAME"
	echo "                     $SYSTEMCTL status $CRON_NAME"
fi

echo
echo "==> Installed $PROJECT_NAME to $LIB_DIR"
case ":$PATH:" in
*":$PREFIX/bin:"*) ;;
*) echo "    Note: $PREFIX/bin is not on your PATH." ;;
esac

# Thumbnail generation shells out to these at runtime: ffmpeg for video
# frames, ghostscript (gs) for PDF first pages. They are optional; without
# them $PROJECT_NAME simply skips thumbnails for those file types.
MISSING_DEPS=()
command -v ffmpeg >/dev/null 2>&1 || MISSING_DEPS+=("ffmpeg")
command -v gs >/dev/null 2>&1 || MISSING_DEPS+=("ghostscript")
if [ "${#MISSING_DEPS[@]}" -gt 0 ]; then
	echo
	echo "    Note: for thumbnail generation, install: ${MISSING_DEPS[*]}"
	echo "          Debian/Ubuntu:  sudo apt install ${MISSING_DEPS[*]}"
	echo "          Fedora:         sudo dnf install ${MISSING_DEPS[*]}"
	echo "          macOS (brew):   brew install ${MISSING_DEPS[*]}"
fi

if [ "$MODE" = biz ] && [ ! -f "$HOME/s3_config.yaml" ]; then
	echo "    Note: biz mode reads S3 credentials from ~/s3_config.yaml, which does not exist yet."
fi

if [ "$SYSTEMD" -eq 0 ]; then
	echo
	echo "    Start it with:  $PROJECT_NAME server -m $MODE"
	echo "    Then open:      http://localhost:8080"
	echo
	echo "    For thumbnails and other periodic jobs, also run:"
	echo "                    $CRON_NAME -m $MODE"
fi
