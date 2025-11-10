#!/bin/bash
#bash <(curl -Ls https://raw.githubusercontent.com/dimasmir03/corvin-ui/main/install.sh)
set -e

echo "Installing panel..."

ARCH=$(uname -m)
VERSION=${1:-latest}
INSTALL_DIR="/usr/local/panel"

mkdir -p $INSTALL_DIR

# Download latest version
if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -s https://api.github.com/repos/dimasmir03/corvin-ui/releases/latest | grep '"tag_name"' | cut -d '"' -f4)
fi

echo "Version: $VERSION"
wget -O /tmp/panel.tar.gz https://github.com/dimasmir03/corvin-ui/releases/download/${VERSION}/panel-linux-amd64.tar.gz
tar -xzf /tmp/panel.tar.gz -C $INSTALL_DIR
chmod +x $INSTALL_DIR/panel

# Systemd service
cat >/etc/systemd/system/panel.service <<EOF
[Unit]
Description=Corvin Panel
After=network.target

[Service]
ExecStart=$INSTALL_DIR/panel
Restart=always
User=root

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable panel
systemctl restart panel

echo "✅ Installed successfully!"
