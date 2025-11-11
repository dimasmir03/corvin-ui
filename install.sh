#!/bin/bash
#bash <(curl -Ls https://raw.githubusercontent.com/dimasmir03/corvin-ui/main/install.sh)
set -e

echo "Installing panel CORVIN-UI..."

ARCH=$(uname -m)
VERSION=${1:-latest}
INSTALL_DIR="/usr/local/corvin-ui"

mkdir -p $INSTALL_DIR

# Download latest version
if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -s https://api.github.com/repos/dimasmir03/corvin-ui/releases/latest | grep '"tag_name"' | cut -d '"' -f4)
fi

echo "Version: $VERSION"
wget -O /tmp/corvin-ui.tar.gz https://github.com/dimasmir03/corvin-ui/releases/download/${VERSION}/corvin-ui-linux-amd64.tar.gz
tar -xzf /tmp/corvin-ui.tar.gz -C $INSTALL_DIR
chmod +x $INSTALL_DIR/corvin-ui

# Systemd service
cat >/etc/systemd/system/corvin-ui.service <<EOF
[Unit]
Description=Corvin-ui Panel
After=network.target

[Service]
ExecStart=$INSTALL_DIR/corvin-ui
Restart=always
User=root

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable corvin-ui
systemctl restart corvin-ui

echo "✅ Installed successfully!"
