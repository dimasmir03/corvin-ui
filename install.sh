#!/bin/bash
# bash <(curl -Ls https://raw.githubusercontent.com/dimasmir03/corvin-ui/main/install.sh)
set -e

APP_NAME="corvin-ui"
VERSION=${1:-latest}
INSTALL_DIR="/usr/local/${APP_NAME}"
SERVICE_FILE="/etc/systemd/system/${APP_NAME}.service"
ENV_DIR="/etc/${APP_NAME}"
ENV_FILE="${ENV_DIR}/${APP_NAME}.env"
LOG_PATH="/var/log/${APP_NAME}"
REPO="dimasmir03/corvin-ui"

random_secret() {
  tr -dc 'A-Za-z0-9' < /dev/urandom | head -c "${1:-32}"
}

create_env_file() {
  mkdir -p "${ENV_DIR}"

  if [ -f "${ENV_FILE}" ]; then
    echo "Keeping existing config: ${ENV_FILE}"
    return
  fi

  DB_PASSWORD=$(random_secret 32)
  RABBITMQ_PASSWORD=$(random_secret 32)
  MINIO_SECRET_KEY=$(random_secret 40)
  SESSION_SECRET=$(random_secret 64)
	MOBILE_JWT_SECRET=$(random_secret 64)

  cat > "${ENV_FILE}" <<EOF
HTTP_ADDR=127.0.0.1:8080
AUTH_MODE=none
ONLINE_COLLECT_INTERVAL=30s

DB_HOST=127.0.0.1
DB_PORT=5432
DB_USER=corvinvpn
DB_PASSWORD=${DB_PASSWORD}
DB_NAME=corvinvpn
DB_SSLMODE=disable

RABBITMQ_USER=corvinvpn
RABBITMQ_PASSWORD=${RABBITMQ_PASSWORD}
RABBITMQ_URL=amqps://corvinvpn:${RABBITMQ_PASSWORD}@127.0.0.1:1765/
RABBITMQ_CONTAINER_URL=amqps://corvinvpn:${RABBITMQ_PASSWORD}@rabbitmq:5671/
AMQP_EXCHANGE_COMPLAINTS=vpn.complaints
AMQP_EXCHANGE_USERS=vpn.users
RABBITMQ_RESULT_QUEUE=corvin.job.results
AMQP_EXCHANGE_COMMANDS=corvin.job.commands
RABBITMQ_EVENTS_EXCHANGE=corvin.agent.events
RABBITMQ_EVENTS_QUEUE=corvin.agent.events.panel
RABBITMQ_EVENTS_ROUTING_KEY=node.snapshot

MINIO_ENDPOINT=127.0.0.1:9000
MINIO_ACCESS_KEY=corvinvpn
MINIO_SECRET_KEY=${MINIO_SECRET_KEY}
MINIO_USE_SSL=false
MINIO_REGION=us-east-1
MINIO_BUCKET=complaints

SESSION_SECRET=${SESSION_SECRET}

MOBILE_API_ENABLED=false
MOBILE_JWT_SECRET=${MOBILE_JWT_SECRET}
MOBILE_ACCESS_TTL_SECONDS=900
MOBILE_REFRESH_TTL_HOURS=720
MOBILE_LOGIN_TTL_MINUTES=10
MOBILE_TELEGRAM_BOT_USERNAME=
MOBILE_PUBLIC_BASE_URL=
MOBILE_CONNECTIVITY_CHECK_URL=
MOBILE_MIN_APP_VERSION=1.0.0
MOBILE_TRUSTED_PROXIES=

CERT_FILE=/opt/corvin-ui/cert/cert.pem
KEY_FILE=/opt/corvin-ui/cert/key.pem
CA_FILE=/opt/corvin-ui/cert/ca.pem
BASE_URL=http://127.0.0.1:8080
CORVIN_UI_ENV_FILE=${ENV_FILE}
EOF

  chmod 600 "${ENV_FILE}"
  echo "Generated config: ${ENV_FILE}"
}

install_cli_wrapper() {
  wget -O "/usr/bin/${APP_NAME}" "https://raw.githubusercontent.com/${REPO}/main/corvin-ui.sh"
  chmod +x "/usr/bin/${APP_NAME}"

  mkdir -p "/opt/${APP_NAME}"
  if [ -f "${INSTALL_DIR}/${APP_NAME}/docker-compose.yml" ]; then
    cp "${INSTALL_DIR}/${APP_NAME}/docker-compose.yml" "/opt/${APP_NAME}/docker-compose.yml"
  else
    wget -O "/opt/${APP_NAME}/docker-compose.yml" "https://raw.githubusercontent.com/${REPO}/main/docker-compose.yml"
  fi
}

ensure_mobile_env() {
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
    echo "Added disabled Mobile API defaults to existing config"
  fi
}

write_service_file() {
  cat > "${SERVICE_FILE}" <<EOF
[Unit]
Description=Corvin UI Panel
After=network.target
Wants=network.target

[Service]
Type=simple
EnvironmentFile=${ENV_FILE}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/${APP_NAME}/${APP_NAME}
Restart=on-failure
RestartSec=5s
User=root

[Install]
WantedBy=multi-user.target
EOF
}

echo "Installing panel CORVIN-UI..."

ARCH=$(uname -m)
case "${ARCH}" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported arch: ${ARCH}" && exit 1 ;;
esac

mkdir -p "${LOG_PATH}"
mkdir -p "${INSTALL_DIR}"
create_env_file
ensure_mobile_env

if [ "${VERSION}" = "latest" ]; then
  VERSION=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d '"' -f4)
fi

echo "Version: ${VERSION}"
wget -O /tmp/corvin-ui.tar.gz "https://github.com/${REPO}/releases/download/${VERSION}/corvin-ui-linux-${ARCH}.tar.gz"
tar -xzf /tmp/corvin-ui.tar.gz -C "${INSTALL_DIR}"
chmod +x "${INSTALL_DIR}/${APP_NAME}/${APP_NAME}"
echo "${VERSION}" > "${INSTALL_DIR}/VERSION"

install_cli_wrapper
/usr/bin/${APP_NAME} init
write_service_file

if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  /usr/bin/${APP_NAME} compose up -d
else
  echo "Docker Compose is not available; PostgreSQL, RabbitMQ and MinIO were not started."
fi

systemctl daemon-reload
systemctl enable "${APP_NAME}"
systemctl restart "${APP_NAME}"

echo "Installed successfully!"
echo "Config: ${ENV_FILE}"
echo "Use SSH tunnel to access the panel:"
echo "ssh -L 8080:127.0.0.1:8080 root@SERVER_IP"
