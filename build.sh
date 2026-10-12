#!/usr/bin/env sh
# Rebuild resources and the Windows exe. Usage: ./build.sh [version]
set -e
VERSION="${1:-1.0.0}"
cd "$(dirname "$0")"
python3 tools/mkres.py app.ico app.manifest "$VERSION" rsrc_windows_amd64.syso
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H windowsgui -s -w" -o AudioSwitcher.exe .
echo "Built AudioSwitcher.exe ($VERSION)"
