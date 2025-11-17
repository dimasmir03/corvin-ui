#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

SERVICE_NAME="corvin-ui"
INSTALL_DIR="/usr/local/${SERVICE_NAME}"
BIN_PATH="${INSTALL_DIR}/${SERVICE_NAME}"
SYSTEMD_PATH="/etc/systemd/system/${SERVICE_NAME}.service"

set -e

# -------------------------------------------------------------------
# Проверка root
# -------------------------------------------------------------------
[[ $EUID -ne 0 ]] && echo -e "${red}Error:${plain} run as root" && exit 1

# -------------------------------------------------------------------
# Определить архитектуру
# -------------------------------------------------------------------
arch() {
    case "$(uname -m)" in
        x86_64) echo "amd64" ;;
        aarch64) echo "arm64" ;;
        *) echo -e "${red}Unsupported arch${plain}" && exit 1 ;;
    esac
}

ARCH=$(arch)

# -------------------------------------------------------------------
# Инсталляция зависимостей
# -------------------------------------------------------------------
install_base() {
    if command -v apt >/dev/null 2>&1; then
        apt update && apt install -y wget curl tar
    elif command -v dnf >/dev/null 2>&1; then
        dnf install -y wget curl tar
    elif command -v yum >/dev/null 2>&1; then
        yum install -y wget curl tar
    elif command -v apk >/dev/null 2>&1; then
        apk add wget curl tar
    else
        echo "Unsupported OS"
        exit 1
    fi
}

# -------------------------------------------------------------------
# Установка панели
# -------------------------------------------------------------------
install_panel() {
    echo -e "${green}Installing ${SERVICE_NAME}...${plain}"

    rm -rf "${INSTALL_DIR}"
    mkdir -p "${INSTALL_DIR}"

    echo -e "${yellow}Downloading latest release...${plain}"

    VERSION=$(curl -s "https://api.github.com/repos/YOUR_GITHUB_USER/YOUR_REPO/releases/latest" \
        | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    wget -O "${INSTALL_DIR}/${SERVICE_NAME}.tar.gz" \
        "https://github.com/YOUR_GITHUB_USER/YOUR_REPO/releases/download/${VERSION}/${SERVICE_NAME}-linux-${ARCH}.tar.gz"

    tar -xzf "${INSTALL_DIR}/${SERVICE_NAME}.tar.gz" -C "${INSTALL_DIR}"
    rm -f "${INSTALL_DIR}/${SERVICE_NAME}.tar.gz"

    chmod +x "${BIN_PATH}"

    # -------------------------------------------------------------------
    # Создание systemd сервиса
    # -------------------------------------------------------------------
    cat > "${SYSTEMD_PATH}" <<EOF
[Unit]
Description=Corvin UI Panel
After=network.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${BIN_PATH}
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable "${SERVICE_NAME}"
    systemctl restart "${SERVICE_NAME}"

    echo -e "${green}Installed successfully!${plain}"
    echo -e "Use: ${yellow}${SERVICE_NAME}${plain} to manage panel"
}

# -------------------------------------------------------------------
# Меню
# -------------------------------------------------------------------
show_menu() {
    echo -e "
${green}${SERVICE_NAME} management${plain}

Usage: ${yellow}${SERVICE_NAME} <command>${plain}

Commands:
  install       Install panel
  uninstall     Remove panel
  update        Update panel
  start         Start service
  stop          Stop service
  restart       Restart service
  status        View status
  log           Show logs
"
}

# -------------------------------------------------------------------
# Команды
# -------------------------------------------------------------------
case "$1" in
    install)
        install_base
        install_panel
        ;;
    uninstall)
        systemctl stop "${SERVICE_NAME}"
        systemctl disable "${SERVICE_NAME}"
        rm -f "${SYSTEMD_PATH}"
        rm -rf "${INSTALL_DIR}"
        systemctl daemon-reload
        echo -e "${green}Uninstalled.${plain}"
        ;;
    update)
        bash <(curl -Ls https://raw.githubusercontent.com/dimasmir03/corvin-ui/main/install.sh)
        ;;
    start)
        systemctl start "${SERVICE_NAME}"
        ;;
    stop)
        systemctl stop "${SERVICE_NAME}"
        ;;
    restart)
        systemctl restart "${SERVICE_NAME}"
        ;;
    status)
        systemctl status "${SERVICE_NAME}"
        ;;
    log)
        journalctl -u "${SERVICE_NAME}" -f
        ;;
    *)
        show_menu
        ;;
esac
