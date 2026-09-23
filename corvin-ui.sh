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
COMPOSE_FILE="/opt/${SERVICE_NAME}/docker-compose.yml"

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
  init         Create /etc/corvin-ui/corvin-ui.env once
  compose ...  Run Docker Compose with the shared env file
  status       Show systemd service status
  start        Start systemd service
  stop         Stop systemd service
  restart      Restart systemd service
  logs         Follow systemd journal logs
  update       Safely upgrade to the latest release (alias for upgrade latest)
  upgrade VER  Backup DB, install a release and rollback binary on failed readiness
  tunnel       Print SSH tunnel example
  env          Show env file path and masked keys
  version      Show wrapper and installed binary info
  doctor       Run installation diagnostics
  settings     Show or update DB-backed panel settings
  install      Install panel (compatibility command)
  uninstall    Remove panel
EOF
}

random_secret() {
    tr -dc 'A-Za-z0-9' < /dev/urandom | head -c "${1:-32}" || true
}

cmd_init() {
    need_root
    if [[ -f "${ENV_FILE}" ]]; then
        ok "already initialized: ${ENV_FILE}"
        return
    fi

    local env_dir db_password rabbitmq_password minio_secret session_secret mobile_jwt_secret
    env_dir=$(dirname "${ENV_FILE}")
    db_password=$(random_secret 32)
    rabbitmq_password=$(random_secret 32)
    minio_secret=$(random_secret 40)
    session_secret=$(random_secret 64)
	mobile_jwt_secret=$(random_secret 64)

    install -d -m 700 "${env_dir}"
    umask 077
    cat > "${ENV_FILE}" <<EOF
HTTP_ADDR=127.0.0.1:8080
AUTH_MODE=none
ONLINE_COLLECT_INTERVAL=30s

DB_HOST=127.0.0.1
DB_PORT=5432
DB_USER=corvinvpn
DB_PASSWORD=${db_password}
DB_NAME=corvinvpn
DB_SSLMODE=disable

RABBITMQ_USER=corvinvpn
RABBITMQ_PASSWORD=${rabbitmq_password}
RABBITMQ_URL=amqps://corvinvpn:${rabbitmq_password}@127.0.0.1:1765/
RABBITMQ_CONTAINER_URL=amqps://corvinvpn:${rabbitmq_password}@rabbitmq:5671/
AMQP_EXCHANGE_COMPLAINTS=vpn.complaints
AMQP_EXCHANGE_USERS=vpn.users
AMQP_EXCHANGE_COMMANDS=corvin.job.commands
RABBITMQ_RESULT_QUEUE=corvin.job.results
RABBITMQ_EVENTS_EXCHANGE=corvin.agent.events
RABBITMQ_EVENTS_QUEUE=corvin.agent.events.panel
RABBITMQ_EVENTS_ROUTING_KEY=node.snapshot

MINIO_ENDPOINT=127.0.0.1:9000
MINIO_ACCESS_KEY=corvinvpn
MINIO_SECRET_KEY=${minio_secret}
MINIO_USE_SSL=false
MINIO_REGION=us-east-1
MINIO_BUCKET=complaints

SESSION_SECRET=${session_secret}
MOBILE_API_ENABLED=false
MOBILE_JWT_SECRET=${mobile_jwt_secret}
MOBILE_ACCESS_TTL_SECONDS=900
MOBILE_REFRESH_TTL_HOURS=720
MOBILE_LOGIN_TTL_MINUTES=10
MOBILE_TELEGRAM_BOT_USERNAME=
MOBILE_PUBLIC_BASE_URL=
MOBILE_CONNECTIVITY_CHECK_URL=
MOBILE_MIN_APP_VERSION=1.0.0
MOBILE_TRUSTED_PROXIES=
TELEGRAM_ENABLED=false
TELEGRAM_BOT_TOKEN=
TELEGRAM_ADMIN_IDS=
TELEGRAM_PROXY_URL=

CERT_FILE=/opt/corvin-ui/cert/cert.pem
KEY_FILE=/opt/corvin-ui/cert/key.pem
CA_FILE=/opt/corvin-ui/cert/ca.pem
BASE_URL=http://127.0.0.1:8080
CORVIN_UI_ENV_FILE=${ENV_FILE}
EOF
    chmod 600 "${ENV_FILE}"
    ok "initialized: ${ENV_FILE}"
}

ensure_mobile_env() {
    [[ -f "${ENV_FILE}" ]] || return 0
    if ! grep -q '^MOBILE_JWT_SECRET=' "${ENV_FILE}"; then
        {
            echo
            echo "MOBILE_API_ENABLED=false"
            echo "MOBILE_JWT_SECRET=$(random_secret 64)"
            echo "MOBILE_ACCESS_TTL_SECONDS=900"
            echo "MOBILE_REFRESH_TTL_HOURS=720"
            echo "MOBILE_LOGIN_TTL_MINUTES=10"
            echo "MOBILE_TELEGRAM_BOT_USERNAME="
            echo "MOBILE_PUBLIC_BASE_URL="
            echo "MOBILE_CONNECTIVITY_CHECK_URL="
            echo "MOBILE_MIN_APP_VERSION=1.0.0"
            echo "MOBILE_TRUSTED_PROXIES="
        } >> "${ENV_FILE}"
        chmod 600 "${ENV_FILE}"
        warn "added disabled Mobile API defaults; configure and enable them when ready"
    fi
}

cmd_compose() {
    need_root
    if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
        fail "Docker Compose v2 is not available"
        exit 1
    fi
    if [[ ! -f "${ENV_FILE}" ]]; then
        cmd_init
    fi
    if [[ ! -f "${COMPOSE_FILE}" ]]; then
        fail "compose file missing: ${COMPOSE_FILE}"
        exit 1
    fi
    if [[ $# -eq 0 ]]; then
        echo "Usage: ${SERVICE_NAME} compose <docker compose arguments>"
        exit 1
    fi
    docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" "$@"
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

resolve_release_version() {
    local requested="${1:-latest}"
    if [[ "${requested}" != "latest" ]]; then
        [[ "${requested}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { fail "invalid version: ${requested}"; exit 1; }
        echo "${requested}"
        return
    fi
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1
}

backup_database() {
    local version="$1" backup_dir="/var/backups/${SERVICE_NAME}" backup_file
    install -d -m 700 "${backup_dir}"
    backup_file="${backup_dir}/pre-${version}-$(date -u +%Y%m%dT%H%M%SZ).sql.gz"
    set -a
    # shellcheck disable=SC1090
    source "${ENV_FILE}"
    set +a
    if container_running postgres; then
        docker exec -e PGPASSWORD="${DB_PASSWORD}" postgres pg_dump -U "${DB_USER}" "${DB_NAME}" | gzip -9 > "${backup_file}"
    elif command -v pg_dump >/dev/null 2>&1; then
        PGPASSWORD="${DB_PASSWORD}" pg_dump -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" "${DB_NAME}" | gzip -9 > "${backup_file}"
    else
        fail "cannot back up PostgreSQL: neither postgres container nor pg_dump is available"
        return 1
    fi
    test -s "${backup_file}" || { fail "database backup is empty"; return 1; }
    ok "database backup: ${backup_file}"
}

cmd_upgrade() {
    need_root
    [[ -f "${ENV_FILE}" ]] || { fail "env file missing: ${ENV_FILE}"; exit 1; }
	ensure_mobile_env
    local requested="${1:-latest}" version arch temp_dir archive checksum staging current rollback was_active=0
    version=$(resolve_release_version "${requested}")
    [[ -n "${version}" ]] || { fail "could not resolve release version"; exit 1; }
    arch=$(uname -m)
    case "${arch}" in x86_64) arch=amd64;; aarch64|arm64) arch=arm64;; *) fail "unsupported upgrade architecture: ${arch}"; exit 1;; esac
    temp_dir=$(mktemp -d)
    archive="${temp_dir}/corvin-ui-linux-${arch}.tar.gz"
    checksum="${archive}.sha256"
    trap 'rm -rf "${temp_dir}"' EXIT
    if [[ -n "${CORVIN_UPGRADE_ARCHIVE:-}" ]]; then
        cp "${CORVIN_UPGRADE_ARCHIVE}" "${archive}"
        cp "${CORVIN_UPGRADE_CHECKSUM:?CORVIN_UPGRADE_CHECKSUM is required}" "${checksum}"
    else
        curl -fL -o "${archive}" "https://github.com/${REPO}/releases/download/${version}/corvin-ui-linux-${arch}.tar.gz"
        curl -fL -o "${checksum}" "https://github.com/${REPO}/releases/download/${version}/corvin-ui-linux-${arch}.tar.gz.sha256"
    fi
    (cd "${temp_dir}" && sha256sum -c "$(basename "${checksum}")")
    tar -xzf "${archive}" -C "${temp_dir}"
    [[ -x "${temp_dir}/${SERVICE_NAME}/${SERVICE_NAME}" ]] || { fail "release archive does not contain executable"; exit 1; }
    backup_database "${version}"
    staging="${INSTALL_DIR}/.${SERVICE_NAME}-${version}.staging"
    current="${INSTALL_DIR}/${SERVICE_NAME}"
    rollback="${INSTALL_DIR}/.${SERVICE_NAME}.rollback"
    rm -rf "${staging}"
    cp -a "${temp_dir}/${SERVICE_NAME}" "${staging}"
    systemctl is-active --quiet "${SERVICE_NAME}" && was_active=1 || true
    systemctl stop "${SERVICE_NAME}" || true
    rm -rf "${rollback}"
    [[ ! -d "${current}" ]] || mv "${current}" "${rollback}"
    mv "${staging}" "${current}"
    chmod +x "${BIN_PATH}"
    cp "${current}/docker-compose.yml" "${COMPOSE_FILE}"
    cp "${current}/corvin-ui.service" "${SYSTEMD_PATH}"
    systemctl daemon-reload
    systemctl restart "${SERVICE_NAME}"
    local ready=0
    for _ in $(seq 1 30); do
        if curl -fsS "http://127.0.0.1:8080/ready" >/dev/null; then ready=1; break; fi
        sleep 1
    done
    if [[ ${ready} -ne 1 ]]; then
        fail "new release did not become ready; rolling binary back"
        systemctl stop "${SERVICE_NAME}" || true
        mv "${current}" "${current}.failed-${version}"
        if [[ -d "${rollback}" ]]; then mv "${rollback}" "${current}"; fi
        if [[ ${was_active} -eq 1 ]]; then systemctl restart "${SERVICE_NAME}"; fi
        exit 1
    fi
    echo "${version}" > "${VERSION_FILE}"
    cp "${current}/corvin-ui.sh" "/usr/bin/${SERVICE_NAME}"
    chmod +x "/usr/bin/${SERVICE_NAME}"
    rm -rf "${rollback}"
    rm -rf "${temp_dir}"
    trap - EXIT
    ok "upgraded to ${version}; migrations and readiness check completed"
}

cmd_update() {
    cmd_upgrade latest
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
    init)
        cmd_init
        ;;
    compose)
        shift
        cmd_compose "$@"
        ;;
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
    upgrade)
        shift
        cmd_upgrade "${1:-latest}"
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
