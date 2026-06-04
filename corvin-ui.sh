#!/bin/bash

set -euo pipefail

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

SERVICE_NAME="corvin-ui"
REPO="dimasmir03/corvin-ui"
INSTALL_DIR="/usr/local/${SERVICE_NAME}"
BIN_PATH="${INSTALL_DIR}/${SERVICE_NAME}/${SERVICE_NAME}"
SYSTEMD_PATH="/etc/systemd/system/${SERVICE_NAME}.service"
ENV_FILE="/etc/${SERVICE_NAME}/${SERVICE_NAME}.env"
LOG_FILE="/var/log/${SERVICE_NAME}/vpnpanel.log"
VERSION_FILE="${INSTALL_DIR}/VERSION"

need_root() {
    if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
        echo -e "${red}Error:${plain} run as root"
        exit 1
    fi
}

ok() {
    echo -e "${green}OK${plain} $1"
}

warn() {
    echo -e "${yellow}WARN${plain} $1"
}

fail() {
    echo -e "${red}FAIL${plain} $1"
}

run_installer() {
    need_root
    bash <(curl -Ls "https://raw.githubusercontent.com/${REPO}/main/install.sh")
}

show_menu() {
    cat <<EOF
${SERVICE_NAME} management

Usage: ${SERVICE_NAME} <command>

Commands:
  status       Show systemd service status
  start        Start systemd service
  stop         Stop systemd service
  restart      Restart systemd service
  logs         Follow systemd journal logs
  update       Update panel through install.sh
  tunnel       Print SSH tunnel example
  env          Show env file path and masked keys
  version      Show wrapper and installed binary info
  doctor       Run installation diagnostics
  settings     Show or update DB-backed panel settings
  install      Install panel (compatibility command)
  uninstall    Remove panel
EOF
}

cmd_status() {
    systemctl status "${SERVICE_NAME}"
}

cmd_start() {
    need_root
    systemctl start "${SERVICE_NAME}"
}

cmd_stop() {
    need_root
    systemctl stop "${SERVICE_NAME}"
}

cmd_restart() {
    need_root
    systemctl restart "${SERVICE_NAME}"
}

cmd_logs() {
    if command -v journalctl >/dev/null 2>&1; then
        journalctl -u "${SERVICE_NAME}" -f
        return
    fi

    if [[ -f "${LOG_FILE}" ]]; then
        tail -f "${LOG_FILE}"
        return
    fi

    fail "journalctl is not available and ${LOG_FILE} does not exist"
    exit 1
}

cmd_update() {
    run_installer
}

cmd_tunnel() {
    echo "ssh -L 8080:127.0.0.1:8080 root@SERVER_IP"
}

mask_value() {
    local value="$1"

    if [[ -z "${value}" ]]; then
        echo "<empty>"
    elif [[ ${#value} -le 4 ]]; then
        echo "****"
    else
        echo "${value:0:2}****${value: -2}"
    fi
}

cmd_env() {
    echo "Env file: ${ENV_FILE}"

    if [[ ! -f "${ENV_FILE}" ]]; then
        warn "env file does not exist"
        return 1
    fi

    while IFS='=' read -r key value; do
        [[ -z "${key}" || "${key}" =~ ^[[:space:]]*# ]] && continue
        key="${key#export }"
        key="${key//[[:space:]]/}"
        value="${value%%#*}"
        value="${value%$'\r'}"
        value="${value#\"}"
        value="${value%\"}"
        value="${value#\'}"
        value="${value%\'}"

        printf '%s=%s\n' "${key}" "$(mask_value "${value}")"
    done < "${ENV_FILE}"
}

cmd_version() {
    echo "${SERVICE_NAME} wrapper: local"

    if [[ -f "${VERSION_FILE}" ]]; then
        echo "installed version: $(cat "${VERSION_FILE}")"
    else
        echo "installed version: unknown"
    fi

    if [[ -x "${BIN_PATH}" ]]; then
        echo "binary: ${BIN_PATH}"
    else
        echo "binary: missing (${BIN_PATH})"
    fi
}

container_running() {
    local name="$1"
    docker ps --format '{{.Names}}' 2>/dev/null | grep -Fxq "${name}"
}

container_exists() {
    local name="$1"
    docker ps -a --format '{{.Names}}' 2>/dev/null | grep -Fxq "${name}"
}

cmd_doctor() {
    local failed=0

    if [[ -f "${ENV_FILE}" ]]; then
        ok "env file exists: ${ENV_FILE}"
    else
        fail "env file missing: ${ENV_FILE}"
        failed=1
    fi

    if [[ -x "${BIN_PATH}" ]]; then
        ok "binary exists: ${BIN_PATH}"
    else
        fail "binary missing or not executable: ${BIN_PATH}"
        failed=1
    fi

    if [[ -f "${SYSTEMD_PATH}" ]]; then
        ok "systemd service file exists: ${SYSTEMD_PATH}"
    else
        fail "systemd service file missing: ${SYSTEMD_PATH}"
        failed=1
    fi

    if command -v systemctl >/dev/null 2>&1; then
        if systemctl is-active --quiet "${SERVICE_NAME}"; then
            ok "${SERVICE_NAME} is active"
        else
            fail "${SERVICE_NAME} is not active"
            failed=1
        fi
    else
        warn "systemctl is not available"
    fi

    if command -v docker >/dev/null 2>&1; then
        ok "docker is available"
        for container in postgres rabbitmq minio; do
            if container_running "${container}"; then
                ok "container ${container} is running"
            elif container_exists "${container}"; then
                fail "container ${container} exists but is not running"
                failed=1
            else
                warn "container ${container} was not found"
            fi
        done
    else
        warn "docker is not available"
    fi

    return "${failed}"
}

cmd_settings() {
    case "${1:-}" in
        show)
            if [[ ! -x "${BIN_PATH}" ]]; then
                fail "binary missing: ${BIN_PATH}"
                exit 1
            fi
            "${BIN_PATH}" settings show
            ;;
        update)
            if [[ ! -x "${BIN_PATH}" ]]; then
                fail "binary missing: ${BIN_PATH}"
                exit 1
            fi
            if [[ $# -lt 3 ]]; then
                echo "Usage: ${SERVICE_NAME} settings update <field> <value>"
                exit 1
            fi
            "${BIN_PATH}" settings update "$2" "$3"
            ;;
        *)
            echo "Usage: ${SERVICE_NAME} settings <show|update> [field] [value]"
            exit 1
            ;;
    esac
}

case "${1:-}" in
    install)
        run_installer
        ;;
    uninstall)
        need_root
        systemctl stop "${SERVICE_NAME}" || true
        systemctl disable "${SERVICE_NAME}" || true
        rm -f "${SYSTEMD_PATH}"
        rm -rf "${INSTALL_DIR}"
        systemctl daemon-reload
        echo -e "${green}Uninstalled.${plain}"
        ;;
    update)
        cmd_update
        ;;
    start)
        cmd_start
        ;;
    stop)
        cmd_stop
        ;;
    restart)
        cmd_restart
        ;;
    status)
        cmd_status
        ;;
    logs|log)
        cmd_logs
        ;;
    tunnel)
        cmd_tunnel
        ;;
    env)
        cmd_env
        ;;
    version)
        cmd_version
        ;;
    doctor)
        cmd_doctor
        ;;
    settings)
        shift
        cmd_settings "$@"
        ;;
    ""|help|-h|--help)
        show_menu
        ;;
    *)
        echo -e "${red}Unknown command:${plain} $1"
        show_menu
        exit 1
        ;;
esac
