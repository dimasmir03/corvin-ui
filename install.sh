#!/bin/bash
# bash <(curl -Ls https://raw.githubusercontent.com/dimasmir03/corvin-ui/main/install.sh)
set -e

echo "Installing panel CORVIN-UI..."

APP_NAME="corvin-ui"
ARCH=$(uname -m)
VERSION=${1:-latest}
INSTALL_DIR="/usr/local/$APP_NAME"
SERVICE_FILE="/etc/systemd/system/$APP_NAME.service"

mkdir -p $INSTALL_DIR

# Download latest version
if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -s https://api.github.com/repos/dimasmir03/corvin-ui/releases/latest | grep '"tag_name"' | cut -d '"' -f4)
fi

echo "Version: $VERSION"
wget -O /tmp/corvin-ui.tar.gz https://github.com/dimasmir03/corvin-ui/releases/download/${VERSION}/corvin-ui-linux-amd64.tar.gz
tar -xzf /tmp/corvin-ui.tar.gz -C $INSTALL_DIR
chmod +x $INSTALL_DIR

# 5) Install CLI wrapper
wget -O /usr/bin/$APP_NAME https://raw.githubusercontent.com/вшьфыьшк03/corvin-ui/main/corvin-ui.sh
chmod +x /usr/bin/$APP_NAME

# Systemd service
cat > $SERVICE_FILE <<EOF
[Unit]
Description=Corvin-ui Panel
After=network.target

[Service]
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/$APP_NAME
Restart=always
User=root

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable corvin-ui
systemctl restart corvin-ui

echo "✅ Installed successfully!"
