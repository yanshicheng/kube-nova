#!/bin/bash

set -e

# ============================================
# Kube-Nova 基础环境一键部署脚本
# ============================================
# 功能：
# 1. 自动根据存储模式部署 PVC（storageClass 或 nfs）
# 2. 部署所有基础服务（MySQL、Redis、MinIO、Jaeger）
# 3. 初始化数据（上传图片、导入 SQL）
# 4. 显示部署信息和访问方式

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# 存储模式：storageClass 或 nfs
# - storageClass: 使用 Kubernetes StorageClass 动态供应（云环境推荐）
# - nfs: 使用 NFS 静态供应（需要预先配置 NFS 服务器）
STORAGE_MODE="storageClass"  # 可选值: storageClass 或 nfs 需要修改 manifests/basic/pvc-nfs.yaml 文件

#（如修改需同步修改 manifests/basic/ 下的 yaml 文件）
NAMESPACE="kube-nova"
MYSQL_PASSWORD="8VlZ2lvIsKBCYSE3"
REDIS_PASSWORD="2VOSiz0vhGtxaBJb"
MINIO_USER="kube-nova-admin"
MINIO_PASSWORD="KubeNova@2024SecretKey!"
MINIO_BUCKET="kube-nova"

# 路径配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
MANIFESTS_DIR="${PROJECT_ROOT}/manifests/basic"
IMAGE_FILE="${PROJECT_ROOT}/images/kube-nova.png"
SQL_FILE="${PROJECT_ROOT}/sql/db.sql"

# PVC 超时时间（秒）
PVC_TIMEOUT=300

# ============================================
# 工具函数
# ============================================

info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

error() {
    echo -e "${RED}[✗]${NC} $1"
    exit 1
}

warn() {
    echo -e "${YELLOW}[!]${NC} $1"
}

step() {
    echo ""
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}$1${NC}"
    echo -e "${CYAN}========================================${NC}"
}

# ============================================
# 环境检查
# ============================================

check_env() {
    step "第 1 步: 环境检查"

    # 检查 kubectl
    info "检查 kubectl..."
    if ! command -v kubectl &>/dev/null; then
        error "未找到 kubectl 命令，请先安装 kubectl"
    fi
    success "kubectl 已安装"

    # 检查集群连接
    info "检查 Kubernetes 集群连接..."
    if ! kubectl cluster-info &>/dev/null; then
        error "无法连接到 Kubernetes 集群"
    fi
    success "集群连接正常"

    # 检查命名空间是否已存在
    info "检查命名空间..."
    if kubectl get namespace ${NAMESPACE} &>/dev/null; then
        error "命名空间 '${NAMESPACE}' 已存在，请先删除: kubectl delete namespace ${NAMESPACE}"
    fi
    success "命名空间检查通过"

    # 检查存储模式配置
    info "检查存储模式配置..."
    if [[ "${STORAGE_MODE}" != "storageClass" && "${STORAGE_MODE}" != "nfs" ]]; then
        error "STORAGE_MODE 必须是 'storageClass' 或 'nfs'，当前值: ${STORAGE_MODE}"
    fi
    success "存储模式: ${STORAGE_MODE}"

    # 检查必要的文件和目录
    info "检查项目文件..."
    [ -d "${MANIFESTS_DIR}" ] || error "未找到 manifests/basic 目录: ${MANIFESTS_DIR}"
    [ -f "${IMAGE_FILE}" ] || error "未找到图片文件: ${IMAGE_FILE}"
    [ -f "${SQL_FILE}" ] || error "未找到 SQL 文件: ${SQL_FILE}"

    # 检查 PVC 文件
    local pvc_file="${MANIFESTS_DIR}/pvc-${STORAGE_MODE}.yaml"
    if [ ! -f "${pvc_file}" ]; then
        error "未找到 PVC 文件: ${pvc_file}"
    fi
    success "项目文件检查完成"

    # 检查必要的 YAML 文件
    local required_files=("namespace.yaml" "mysql.yaml" "redis.yaml" "minio.yaml" "jaeger.yaml")
    for file in "${required_files[@]}"; do
        [ -f "${MANIFESTS_DIR}/${file}" ] || error "未找到必需文件: ${file}"
    done
    success "所有必需的 YAML 文件都存在"

    # 如果是 NFS 模式，提示用户检查 NFS 配置
    if [ "${STORAGE_MODE}" == "nfs" ]; then
        warn "NFS 模式需要确保："
        warn "  1. NFS 服务器已正确配置并运行"
        warn "  2. pvc-nfs.yaml 中的 NFS 服务器地址和路径已正确配置"
        warn "  3. Kubernetes 节点已安装 NFS 客户端 (nfs-common 或 nfs-utils)"
        warn "  4. NFS 服务器上的目录已创建并有正确的权限"
        echo ""
        read -p "按 Enter 继续，或 Ctrl+C 取消..." dummy
    fi

    success "环境检查完成"
}

# ============================================
# 创建命名空间
# ============================================

create_namespace() {
    step "第 2 步: 创建命名空间"

    info "创建命名空间 ${NAMESPACE}..."
    kubectl apply -f "${MANIFESTS_DIR}/namespace.yaml" || error "命名空间创建失败"
    success "命名空间已创建"
}

# ============================================
# 部署 PVC
# ============================================

deploy_pvc() {
    step "第 3 步: 部署持久化存储 (PVC)"

    local pvc_file="${MANIFESTS_DIR}/pvc-${STORAGE_MODE}.yaml"

    info "使用 PVC 文件: $(basename ${pvc_file})"
    info "存储模式: ${STORAGE_MODE}"

    kubectl apply -f "${pvc_file}" || error "PVC 部署失败"
    success "PVC 资源已提交"
}

# ============================================
# 等待 PVC 绑定
# ============================================

wait_pvc_bound() {
    step "第 4 步: 等待 PVC 绑定"

    # PVC 列表（需要与 yaml 文件中的名称保持一致）
    local pvcs=("jaeger-data" "mysql-pvc" "minio-pvc" "redis-pvc")

    info "等待以下 PVC 绑定: ${pvcs[*]}"

    for pvc in "${pvcs[@]}"; do
        info "等待 ${pvc} 绑定..."

        local waited=0
        while [ $waited -lt ${PVC_TIMEOUT} ]; do
            local status=$(kubectl get pvc ${pvc} -n ${NAMESPACE} -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")

            if [ "$status" == "Bound" ]; then
                success "${pvc} 已绑定"
                break
            elif [ "$status" == "NotFound" ]; then
                # PVC 还未创建，继续等待
                sleep 2
                waited=$((waited + 2))
            elif [ "$status" == "Pending" ]; then
                # PVC 处于 Pending 状态，继续等待
                sleep 2
                waited=$((waited + 2))
            else
                # 未知状态
                warn "${pvc} 状态: ${status}"
                sleep 2
                waited=$((waited + 2))
            fi

            if [ $waited -ge ${PVC_TIMEOUT} ]; then
                error "${pvc} 在 ${PVC_TIMEOUT} 秒内未能绑定，请检查存储配置"
            fi
        done
    done

    # 显示 PVC 状态
    echo ""
    info "所有 PVC 状态："
    kubectl get pvc -n ${NAMESPACE}

    success "所有 PVC 已绑定"
}

# ============================================
# 部署基础服务
# ============================================

deploy_resources() {
    step "第 5 步: 部署基础服务"

    info "部署 MySQL..."
    kubectl apply -f "${MANIFESTS_DIR}/mysql.yaml"
    success "MySQL 已提交"

    info "部署 Redis..."
    kubectl apply -f "${MANIFESTS_DIR}/redis.yaml"
    success "Redis 已提交"

    info "部署 MinIO..."
    kubectl apply -f "${MANIFESTS_DIR}/minio.yaml"
    success "MinIO 已提交"

    info "部署 Jaeger..."
    kubectl apply -f "${MANIFESTS_DIR}/jaeger.yaml"
    success "Jaeger 已提交"

    success "所有服务资源已提交"
}

# ============================================
# 等待 Pod 就绪
# ============================================

wait_pods() {
    step "第 6 步: 等待服务启动"

    local pods=("mysql" "redis" "minio" "jaeger")

    for pod in "${pods[@]}"; do
        info "等待 ${pod} Pod 就绪..."

        if kubectl wait --for=condition=ready pod -l app=${pod} --field-selector=status.phase!=Succeeded,status.phase!=Failed -n ${NAMESPACE} --timeout=300s 2>/dev/null; then
            success "${pod} 已就绪"
        else
            error "${pod} Pod 启动失败，请检查日志: kubectl logs -n ${NAMESPACE} -l app=${pod}"
        fi
    done

    success "所有服务 Pod 已就绪"
}

# ============================================
# 等待 MinIO 初始化
# ============================================

wait_minio_init() {
    step "第 7 步: 等待 MinIO 初始化"

    info "等待 MinIO 桶初始化任务完成..."

    if kubectl wait --for=condition=complete job/minio-init-bucket -n ${NAMESPACE} --timeout=180s 2>/dev/null; then
        success "MinIO 初始化完成"
    else
        warn "MinIO 初始化任务可能失败，请检查日志："
        warn "kubectl logs -n ${NAMESPACE} job/minio-init-bucket"
        error "MinIO 初始化失败"
    fi
}

# ============================================
# 上传图片到 MinIO
# ============================================

upload_image() {
    step "第 8 步: 上传图片到 MinIO"

    info "获取 MinIO Pod..."
    local minio_pod=$(kubectl get pod -n ${NAMESPACE} -l app=minio -o jsonpath='{.items[0].metadata.name}')

    if [ -z "${minio_pod}" ]; then
        error "未找到 MinIO Pod"
    fi

    info "MinIO Pod: ${minio_pod}"
    info "上传图片: $(basename ${IMAGE_FILE})"

    # 将图片转换为 base64（跨平台兼容）
    local image_base64=$(base64 < "${IMAGE_FILE}" | tr -d '\n')

    # 通过 stdin 传输并解码，然后上传到 MinIO
    kubectl exec -n ${NAMESPACE} ${minio_pod} -- sh -c "
        set -e
        echo '上传图片到 MinIO...'
        echo '${image_base64}' | base64 -d > /tmp/kube-nova.png
        mc alias set myminio http://localhost:9000 '${MINIO_USER}' '${MINIO_PASSWORD}'
        mc cp /tmp/kube-nova.png myminio/${MINIO_BUCKET}/public/kube-nova.png
        rm /tmp/kube-nova.png
        echo '图片上传完成'
    " || error "图片上传失败"

    success "图片上传成功: public/kube-nova.png"
}

# ============================================
# 导入 SQL 到 MySQL
# ============================================

import_sql() {
    step "第 9 步: 导入 SQL 到 MySQL"

    info "获取 MySQL Pod..."
    local mysql_pod=$(kubectl get pod -n ${NAMESPACE} -l app=mysql -o jsonpath='{.items[0].metadata.name}')

    if [ -z "${mysql_pod}" ]; then
        error "未找到 MySQL Pod"
    fi

    info "MySQL Pod: ${mysql_pod}"

    # 等待 MySQL 完全启动
    info "等待 MySQL 服务就绪..."
    sleep 10

    # 创建数据库
    info "创建数据库 kube_nova..."
    kubectl exec -n ${NAMESPACE} ${mysql_pod} -- sh -c "
        mysql -uroot -p${MYSQL_PASSWORD} -e 'CREATE DATABASE IF NOT EXISTS kube_nova DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;'
    " || error "创建数据库失败"
    success "数据库创建成功"

    # 导入 SQL 文件
    info "导入 SQL 数据..."
    kubectl exec -i -n ${NAMESPACE} ${mysql_pod} -- mysql -uroot -p${MYSQL_PASSWORD} kube_nova < "${SQL_FILE}" || \
        error "SQL 导入失败"

    success "SQL 导入成功"
}

# ============================================
# 显示部署信息
# ============================================

show_info() {
    step "部署完成 - 服务信息"

    # 获取 Jaeger NodePort
    local jaeger_port=$(kubectl get svc jaeger-ui -n ${NAMESPACE} -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "N/A")

    echo ""
    echo -e "${GREEN}=========================================="
    echo -e " Kube-Nova 基础环境部署完成"
    echo -e "==========================================${NC}"

    # MySQL 信息
    echo -e "\n${CYAN} MySQL 连接信息:${NC}"
    echo -e "  ${BLUE}集群内访问:${NC} mysql.${NAMESPACE}.svc.cluster.local:3306"
    echo -e "  ${BLUE}用户名:${NC} root"
    echo -e "  ${BLUE}密码:${NC} ${MYSQL_PASSWORD}"
    echo -e "  ${BLUE}数据库:${NC} kube_nova"
    echo -e "  ${YELLOW}端口转发:${NC} kubectl port-forward -n ${NAMESPACE} svc/mysql 3306:3306"

    # Redis 信息
    echo -e "\n${CYAN} Redis 连接信息:${NC}"
    echo -e "  ${BLUE}集群内访问:${NC} redis.${NAMESPACE}.svc.cluster.local:6379"
    echo -e "  ${BLUE}密码:${NC} ${REDIS_PASSWORD}"
    echo -e "  ${YELLOW}端口转发:${NC} kubectl port-forward -n ${NAMESPACE} svc/redis 6379:6379"
    echo -e "  ${YELLOW}连接测试:${NC} redis-cli -h localhost -p 6379 -a ${REDIS_PASSWORD} ping"

    # MinIO 信息
    echo -e "\n${CYAN} MinIO 对象存储:${NC}"
    echo -e "  ${BLUE}集群内 API:${NC} minio-service.${NAMESPACE}.svc.cluster.local:9000"
    echo -e "  ${BLUE}集群内控制台:${NC} minio-service.${NAMESPACE}.svc.cluster.local:9001"
    echo -e "  ${BLUE}访问密钥:${NC} ${MINIO_USER}"
    echo -e "  ${BLUE}密钥:${NC} ${MINIO_PASSWORD}"
    echo -e "  ${BLUE}桶名:${NC} ${MINIO_BUCKET} (公开访问)"
    echo -e "  ${BLUE}已上传图片:${NC} public/kube-nova.png"
    echo -e "  ${YELLOW}端口转发:${NC}"
    echo -e "    API:      kubectl port-forward -n ${NAMESPACE} svc/minio-service 9000:9000"
    echo -e "    控制台:    kubectl port-forward -n ${NAMESPACE} svc/minio-service 9001:9001"
    echo -e "  ${YELLOW}访问控制台:${NC} http://localhost:9001"

    # Jaeger 信息
    echo -e "\n${CYAN} Jaeger 链路追踪:${NC}"
    echo -e "  ${BLUE}集群内 Collector:${NC} jaeger-collector.${NAMESPACE}.svc.cluster.local"
    echo -e "  ${BLUE}HTTP Collector:${NC} http://jaeger-collector:14268/api/traces"
    echo -e "  ${BLUE}gRPC Collector:${NC} jaeger-collector:14250"
    echo -e "  ${BLUE}OTLP gRPC:${NC} jaeger-collector:4317"
    echo -e "  ${BLUE}OTLP HTTP:${NC} http://jaeger-collector:4318"

    if [ "${jaeger_port}" != "N/A" ]; then
        echo -e "  ${BLUE}UI NodePort:${NC} ${GREEN}${jaeger_port}${NC}"
        echo -e "  ${YELLOW}访问 UI:${NC} http://<NodeIP>:${jaeger_port}"
    fi
    echo -e "  ${YELLOW}端口转发:${NC} kubectl port-forward -n ${NAMESPACE} svc/jaeger-ui 16686:16686"
    echo -e "  ${YELLOW}访问 UI:${NC} http://localhost:16686"

    # 存储信息
    echo -e "\n${CYAN}存储配置:${NC}"
    echo -e "  ${BLUE}存储模式:${NC} ${STORAGE_MODE}"
    if [ "${STORAGE_MODE}" == "storageClass" ]; then
        echo -e "  ${BLUE}说明:${NC} 使用 Kubernetes StorageClass 动态供应"
    else
        echo -e "  ${BLUE}说明:${NC} 使用 NFS 静态供应"
    fi

    # 常用命令
    echo -e "\n${CYAN}常用命令:${NC}"
    echo -e "  ${YELLOW}查看所有资源:${NC}"
    echo -e "    kubectl get all -n ${NAMESPACE}"
    echo -e "  ${YELLOW}查看 PVC 状态:${NC}"
    echo -e "    kubectl get pvc -n ${NAMESPACE}"
    echo -e "  ${YELLOW}查看 Pod 日志:${NC}"
    echo -e "    kubectl logs -n ${NAMESPACE} <pod-name>"
    echo -e "  ${YELLOW}进入 Pod:${NC}"
    echo -e "    kubectl exec -it -n ${NAMESPACE} <pod-name> -- sh"
    echo -e "  ${YELLOW}删除所有资源:${NC}"
    echo -e "    kubectl delete namespace ${NAMESPACE}"

    echo -e "\n${GREEN}==========================================${NC}"
    echo ""

}

# ============================================
# 清理函数（可选，用于脚本失败时清理）
# ============================================

cleanup_on_error() {
    error "部署过程中发生错误"
    warn "如需清理已部署的资源，请运行:"
    warn "  kubectl delete namespace ${NAMESPACE}"
    exit 1
}

# 设置错误陷阱
trap cleanup_on_error ERR

# ============================================
# 主流程
# ============================================

main() {
    echo ""
    echo -e "${GREEN}=========================================="
    echo -e " Kube-Nova 基础环境一键部署"
    echo -e "==========================================${NC}"
    echo -e "${BLUE}存储模式:${NC} ${STORAGE_MODE}"
    echo -e "${BLUE}命名空间:${NC} ${NAMESPACE}"
    echo -e "${GREEN}==========================================${NC}"
    echo ""

    check_env
    create_namespace
    deploy_pvc
    wait_pvc_bound
    deploy_resources
    wait_pods
    wait_minio_init
    upload_image
    import_sql
    show_info

    echo ""
    success " 部署完成！所有服务已成功启动！"
    echo ""
}

# 执行主流程
main "$@"