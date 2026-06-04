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
    echo "Config already exists: ${ENV_FILE}"
    read -r -p "Overwrite it and generate new secrets? [y/N]: " answer
    case "${answer}" in
      y|Y|yes|YES) ;;
      *)
        echo "Keeping existing config."
        return
        ;;
    esac
  fi

  DB_PASSWORD=$(random_secret 32)
  RABBITMQ_PASSWORD=$(random_secret 32)
  MINIO_SECRET_KEY=$(random_secret 40)
  SESSION_SECRET=$(random_secret 64)

  cat > "${ENV_FILE}" <<EOF
HTTP_ADDR=127.0.0.1:8080
AUTH_MODE=none

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

MINIO_ENDPOINT=127.0.0.1:9000
MINIO_ACCESS_KEY=corvinvpn
MINIO_SECRET_KEY=${MINIO_SECRET_KEY}
MINIO_USE_SSL=false
MINIO_REGION=us-east-1
MINIO_BUCKET=complaints

SESSION_SECRET=${SESSION_SECRET}

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

if [ "${VERSION}" = "latest" ]; then
  VERSION=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d '"' -f4)
fi

echo "Version: ${VERSION}"
wget -O /tmp/corvin-ui.tar.gz "https://github.com/${REPO}/releases/download/${VERSION}/corvin-ui-linux-${ARCH}.tar.gz"
tar -xzf /tmp/corvin-ui.tar.gz -C "${INSTALL_DIR}"
chmod +x "${INSTALL_DIR}/${APP_NAME}/${APP_NAME}"

install_cli_wrapper
write_service_file

systemctl daemon-reload
systemctl enable "${APP_NAME}"
systemctl restart "${APP_NAME}"

echo "Installed successfully!"
echo "Config: ${ENV_FILE}"
echo "Use SSH tunnel to access the panel:"
echo "ssh -L 8080:127.0.0.1:8080 root@SERVER_IP"
