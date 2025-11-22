#!/bin/bash
PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:~/bin

# bash <(curl --insecure -fsSL https://raw.githubusercontent.com/midoks/move86go/main/scripts/debug.sh)

APP_NAME=move86go
TAGRT_DIR=/usr/local/${APP_NAME}_dev
# /usr/local/move86go_dev

mkdir -p $TAGRT_DIR
cd $TAGRT_DIR

export GIT_COMMIT=$(git rev-parse HEAD)
export BUILD_TIME=$(date -u '+%Y-%m-%d %I:%M:%S %Z')


if [ ! -d $TAGRT_DIR/${APP_NAME} ]; then
	git clone https://github.com/midoks/${APP_NAME}
	cd $TAGRT_DIR/${APP_NAME}
else
	cd $TAGRT_DIR/${APP_NAME}
	git pull https://github.com/midoks/${APP_NAME}
fi

rm -rf ./go.sum
rm -rf ./go.mod
go mod init move86go
go mod tidy
go mod vendor


cd $TAGRT_DIR/${APP_NAME}/scripts

systemctl daemon-reload
service ${APP_NAME} restart



# cd /usr/local/move86go_dev/move86go && go build ./ && ./move86go debug

# cd /usr/local/move86go_dev/move86go && go build ./ && setcap cap_net_admin,cap_net_raw+ep ./move86go && systemd-run --unit=move86go-debug -p 'AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW' -p 'CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW' -p 'NoNewPrivileges=yes' ./move86go debug


cd /usr/local/move86go_dev/move86go && go build ./ && setcap cap_net_admin,cap_net_raw+ep ./move86go && ./move86go debug