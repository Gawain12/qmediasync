#!/bin/bash

# QMediaSync 开发环境启动脚本 (优化版)
# 支持版本号注入与自动环境清理

set -e

cd "$(dirname "$0")"

# --- 配置中心 ---
VERSION="v0.0.1-dev"
BUILD_DATE=$(date +'%Y-%m-%d %H:%M:%S')
DB_PORT=15438  # unified-postgres18 端口，与 Docker 部署一致
APP_PORT=12333
# ----------------

echo "=========================================="
echo "  QMediaSync 开发环境启动"
echo "  版本: $VERSION"
echo "  构建时间: $BUILD_DATE"
echo "=========================================="

# 1. 清理占用端口的旧进程
echo "清理端口占用 ($APP_PORT)..."
fuser -k $APP_PORT/tcp 8095/tcp 8094/tcp 12332/tcp 2>/dev/null || true

# 2. 环境健康检查
if docker ps -q -f name=^qmediasync$ | grep -q .; then
    echo "停止冲突的生产容器 (qmediasync)..."
    docker stop qmediasync
fi

if ! docker ps -q -f name=unified-postgres18 | grep -q .; then
    echo "启动数据库容器 (unified-postgres18)..."
    docker start unified-postgres18
    sleep 2
fi

# 3. 设置环境变量
export TZ=Asia/Shanghai
export DB_EXTERNAL=true
export DB_HOST=localhost
export DB_PORT=$DB_PORT
export DB_USER=qms
export DB_PASSWORD=qms123456
export DB_NAME=qms
export DB_SSLMODE=disable

echo ""
echo "状态检查:"
echo "  - 数据库: $DB_HOST:$DB_PORT (已就绪)"
echo "  - 访问地址: http://$(hostname -I | awk '{print $1}'):$APP_PORT"
echo ""
echo "=========================================="
echo ""

# 4. 启动应用 (注入版本信息)
go run -ldflags " \
    -X 'main.Version=$VERSION' \
    -X 'main.PublishDate=$BUILD_DATE' \
" main.go "$@"
