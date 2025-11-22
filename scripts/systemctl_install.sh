#!/bin/bash
PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin
export PATH

APP_NAME=move86go
INSTALL_DIR=/opt/${APP_NAME}

if [ "$EUID" -ne 0 ]; then
  echo "need root"
  exit 1
fi

mkdir -p ${INSTALL_DIR}

if [ -f ./${APP_NAME} ]; then
  cp -f ./${APP_NAME} ${INSTALL_DIR}/${APP_NAME}
  chmod +x ${INSTALL_DIR}/${APP_NAME}
fi

SYSTEMD_DIR=/etc/systemd/system
if [ -d /lib/systemd/system ]; then
  SYSTEMD_DIR=/lib/systemd/system
elif [ -d /usr/lib/systemd/system ]; then
  SYSTEMD_DIR=/usr/lib/systemd/system
fi

cp -f ./scripts/${APP_NAME}.service ${SYSTEMD_DIR}/${APP_NAME}.service

systemctl daemon-reload
systemctl enable ${APP_NAME}
systemctl restart ${APP_NAME}

echo "installed: ${SYSTEMD_DIR}/${APP_NAME}.service"
echo "workdir: ${INSTALL_DIR}"
