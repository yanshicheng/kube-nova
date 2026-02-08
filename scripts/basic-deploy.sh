#!/bin/bash

# ============================================
# Kube-Nova 基础环境部署脚本 (智能路径 + 强健壮版)
# ============================================

set -e

# --- 颜色定义 ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# --- 基础配置 ---
STORAGE_MODE="storageClass"
NAMESPACE="kube-nova"
MYSQL_PASSWORD="8VlZ2lvIsKBCYSE3"
MINIO_USER="kube-nova-admin"
MINIO_PASSWORD="KubeNova@2024SecretKey!"
MINIO_BUCKET="kube-nova"
PVC_TIMEOUT=300

# --- 工具函数 ---
info() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[✓]${NC} $1"; }
error() { echo -e "${RED}[✗]${NC} $1"; exit 1; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
step() { echo -e "\n${CYAN}>> $1${NC}"; }

# --- 1. 智能路径溯源 ---
find_project_root() {
    step "路径定位"

    # 获取脚本实际所在目录
    local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local current_dir="$(pwd)"

    # 候选根目录：脚本所在目录、脚本所在目录的父目录、当前工作目录
    local candidates=("$script_dir" "$(dirname "$script_dir")" "$current_dir")

    PROJECT_ROOT=""
    for dir in "${candidates[@]}"; do
        if [ -d "$dir/manifests/basic" ]; then
            PROJECT_ROOT="$dir"
            break
        fi
    done

    if [ -z "$PROJECT_ROOT" ]; then
        error "无法定位项目根目录。请确保在项目目录下执行，或 manifests 目录在脚本的上一级。"
    fi

    # 重新定义关键路径
    MANIFESTS_DIR="${PROJECT_ROOT}/manifests/basic"
    IMAGE_FILE="${PROJECT_ROOT}/images/kube-nova.png"
    SQL_FILE="${PROJECT_ROOT}/sql/db.sql"

    info "项目根目录: ${PROJECT_ROOT}"
    info "资源目录: ${MANIFESTS_DIR}"
}

# --- 2. 环境检查 ---
check_env() {
    step "环境检查"
    command -v kubectl &>/dev/null || error "未找到 kubectl"
    kubectl cluster-info &>/dev/null || error "无法连接 K8s 集群"

    [ -f "${MANIFESTS_DIR}/namespace.yaml" ] || error "未找到 namespace.yaml 于 ${MANIFESTS_DIR}"
    [ -f "${SQL_FILE}" ] || error "SQL文件不存在: ${SQL_FILE}"

    if [ ! -f "${IMAGE_FILE}" ]; then
        warn "图片文件不存在: ${IMAGE_FILE}，脚本将跳过上传步骤。"
    fi
}

# --- 3. 部署资源 ---
deploy_resources() {
    step "应用 K8s 配置"
    info "创建命名空间与 PVC..."
    kubectl apply -f "${MANIFESTS_DIR}/namespace.yaml"
    kubectl apply -f "${MANIFESTS_DIR}/pvc-${STORAGE_MODE}.yaml"

    # 使用灵活的等待逻辑
    info "等待 PVC 绑定 (300s)..."
    kubectl wait --for=jsonpath='{.status.phase}'=Bound pvc --all -n ${NAMESPACE} --timeout=${PVC_TIMEOUT}s --allow-missing-template-keys=true

    info "应用核心服务配置..."
    kubectl apply -f "${MANIFESTS_DIR}/mysql.yaml"
    kubectl apply -f "${MANIFESTS_DIR}/redis.yaml"
    kubectl apply -f "${MANIFESTS_DIR}/minio.yaml"
    kubectl apply -f "${MANIFESTS_DIR}/jaeger.yaml"
}

# --- 4. 健壮的服务就绪检查 ---
wait_for_services() {
    step "等待服务就绪"
    local apps=("mysql" "redis" "minio" "jaeger")
    for app in "${apps[@]}"; do
        info "等待 ${app} Pod 达到 Ready 状态..."
        kubectl wait --for=condition=ready pod -l app=${app} -n ${NAMESPACE} --timeout=120s || error "${app} 启动超时"
    done

    info "等待 MySQL 应用层响应 (mysqladmin ping)..."
    local retry=0
    # 使用 deployment 别名定位 Pod，更稳定
    until kubectl exec -n ${NAMESPACE} deployment/mysql -- mysqladmin ping -uroot -p${MYSQL_PASSWORD} --silent; do
        [[ $retry -gt 20 ]] && error "MySQL 进程未能在 60s 内响应"
        sleep 3
        retry=$((retry + 1))
    done
    success "所有核心服务网络连通性正常"
}

# --- 5. 初始化 MinIO (流式传输解决卡死) ---
init_minio_data() {
    step "初始化 MinIO 数据"

    info "检查 MinIO 初始化 Job 状态..."
    # 如果 Job 卡住，60s 后强制继续，由后续 mc 命令进行二次验证
    kubectl wait --for=condition=complete job/minio-init-bucket -n ${NAMESPACE} --timeout=60s 2>/dev/null || warn "MinIO 初始化 Job 未及时完成，尝试手动同步..."

    if [ -f "${IMAGE_FILE}" ]; then
        local minio_pod=$(kubectl get pod -l app=minio -n ${NAMESPACE} -o jsonpath='{.items[0].metadata.name}')
        info "流式上传图片: $(basename ${IMAGE_FILE})"

        # 核心改动：用管道 cat 输入，解决 exec 命令行过长导致的僵死
        cat "${IMAGE_FILE}" | kubectl exec -i -n ${NAMESPACE} "${minio_pod}" -- sh -c "
            mc alias set local http://localhost:9000 ${MINIO_USER} ${MINIO_PASSWORD} --api s3v4 > /dev/null
            cat > /tmp/temp_img.png
            mc cp /tmp/temp_img.png local/${MINIO_BUCKET}/public/kube-nova.png
            rm /tmp/temp_img.png
        " && success "图片上传成功" || warn "图片上传失败"
    fi
}

# --- 6. 导入数据库 ---
init_mysql_data() {
    step "导入数据库数据"
    info "执行 SQL 导入: ${SQL_FILE}"

    # 预创建数据库
    kubectl exec -i -n ${NAMESPACE} deployment/mysql -- mysql -uroot -p${MYSQL_PASSWORD} \
        -e "CREATE DATABASE IF NOT EXISTS kube_nova DEFAULT CHARACTER SET utf8mb4;"

    # 流式导入
    if kubectl exec -i -n ${NAMESPACE} deployment/mysql -- mysql -uroot -p${MYSQL_PASSWORD} kube_nova < "${SQL_FILE}"; then
        success "SQL 数据导入成功"
    else
        error "SQL 导入过程中出错"
    fi
}

# --- 7. 完成信息展示 ---
show_summary() {
    # 尝试获取 NodeIP，如果是在本地集群(Kind/Minikube)可能需要 port-forward
    local node_ip=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || echo "127.0.0.1")
    local jaeger_port=$(kubectl get svc jaeger-ui -n ${NAMESPACE} -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "16686")

    echo -e "\n${GREEN}=========================================="
    echo -e "         Kube-Nova 部署成功"
    echo -e "==========================================${NC}"
    echo -e "${CYAN}Jaeger UI:${NC}  http://${node_ip}:${jaeger_port}"
    echo -e "${CYAN}MySQL 密码:${NC} ${MYSQL_PASSWORD}"
    echo -e "${CYAN}MinIO 桶:${NC}   ${MINIO_BUCKET}/public"
    echo -e "------------------------------------------"
    echo -e "如需本地访问 MinIO 控制台，请执行:"
    echo -e "kubectl port-forward -n ${NAMESPACE} svc/minio-service 9001:9001"
    echo -e "==========================================\n"
}

# --- 主流程 ---
main() {
    find_project_root
    check_env
    deploy_resources
    wait_for_services
    init_minio_data
    init_mysql_data
    show_summary
}

main