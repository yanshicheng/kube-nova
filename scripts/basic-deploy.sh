#!/bin/bash
set -e

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; CYAN='\033[0;36m'; NC='\033[0m'
NAMESPACE="kube-nova"
MYSQL_PASSWORD="8VlZ2lvIsKBCYSE3"
MONGO_PASSWORD="8VlZ2lvIsKBCYSE3"
MINIO_USER="kube-nova-admin"
MINIO_PASSWORD="KubeNova@2024SecretKey!"
MINIO_BUCKET="kube-nova"

info() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
error() { echo -e "${RED}[FAIL]${NC} $1"; exit 1; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
step() { echo -e "\n${CYAN}>> $1${NC}"; }

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
MANIFESTS_DIR="${PROJECT_ROOT}/manifests/basic"
SQL_FILE="${PROJECT_ROOT}/sql/db-init.sql"
MONGO_DIR="${PROJECT_ROOT}/sql/mongo-export"
IMAGE_FILE="${PROJECT_ROOT}/images/kube-nova.png"

step "Kube-Nova 基础环境一键部署"
info "项目目录: ${PROJECT_ROOT}"

command -v kubectl >/dev/null 2>&1 || error "未找到 kubectl"
kubectl get ns kube-system >/dev/null 2>&1 || error "无法连接 K8s 集群"

# ---- clean old ----
if kubectl get ns ${NAMESPACE} >/dev/null 2>&1; then
    step "清理旧环境"
    # 1. 删 NS
    kubectl delete ns ${NAMESPACE} --force --grace-period=0 --wait=false >/dev/null 2>&1 || true
    sleep 10
    # 2. 强制清理残留 PVC（移除 finalizer）
    kubectl get pvc -n ${NAMESPACE} -o name 2>/dev/null | while read pvc; do
        kubectl patch "$pvc" -n ${NAMESPACE} -p '{"metadata":{"finalizers":null}}' >/dev/null 2>&1 || true
    done
    kubectl delete pvc --all -n ${NAMESPACE} --force --grace-period=0 >/dev/null 2>&1 || true
    # 3. 强制清理残留 PV
    kubectl get pv -o name 2>/dev/null | grep ${NAMESPACE} | while read pv; do
        kubectl patch "$pv" -p '{"metadata":{"finalizers":null}}' >/dev/null 2>&1 || true
        kubectl delete "$pv" --force --grace-period=0 >/dev/null 2>&1 || true
    done
    # 4. 等待 NS 完全删除
    for i in $(seq 1 60); do
        kubectl get ns ${NAMESPACE} >/dev/null 2>&1 || break
        sleep 5
    done
    kubectl get ns ${NAMESPACE} >/dev/null 2>&1 && error "无法清理旧命名空间，请手动检查" || true
    success "旧环境已清理"
fi

# ---- deploy ----
step "部署基础资源"
kubectl apply -f "${MANIFESTS_DIR}/namespace.yaml" >/dev/null
kubectl apply -f "${MANIFESTS_DIR}/pvc-storageClass.yaml" >/dev/null
info "等待 PVC 绑定..."
kubectl wait --for=jsonpath='{.status.phase}'=Bound pvc --all -n ${NAMESPACE} --timeout=300s >/dev/null
kubectl apply -f "${MANIFESTS_DIR}/mysql.yaml" >/dev/null
kubectl apply -f "${MANIFESTS_DIR}/redis.yaml" >/dev/null
kubectl apply -f "${MANIFESTS_DIR}/minio.yaml" >/dev/null
kubectl apply -f "${MANIFESTS_DIR}/jaeger.yaml" >/dev/null
kubectl apply -f "${MANIFESTS_DIR}/mongodb.yaml" >/dev/null

# ---- wait ready ----
step "等待服务就绪"
for app in mysql redis minio jaeger mongodb; do
    info "等待 ${app}..."
    kubectl wait --for=condition=ready pod -l "app=${app},!job-name" -n ${NAMESPACE} --timeout=300s >/dev/null || error "${app} 启动超时"
    success "${app} 就绪"
done

# ---- mysql ping + auto-fix ----
info "MySQL 探活..."
MYSQL_OK=0
for i in $(seq 1 30); do
    if kubectl exec -n ${NAMESPACE} deployment/mysql -- mysqladmin ping -uroot -p${MYSQL_PASSWORD} --silent 2>/dev/null; then
        MYSQL_OK=1; break
    fi
    if kubectl exec -n ${NAMESPACE} deployment/mysql -- mysqladmin ping -uroot --silent 2>/dev/null; then
        kubectl exec -n ${NAMESPACE} deployment/mysql -- mysql -uroot -e "ALTER USER 'root'@'localhost' IDENTIFIED BY '${MYSQL_PASSWORD}'; FLUSH PRIVILEGES;" 2>/dev/null
        MYSQL_OK=1; break
    fi
    [ $i -eq 30 ] && error "MySQL 无响应"
    sleep 3
done
[ $MYSQL_OK -eq 1 ] && success "MySQL 正常"

# ---- mongo ping ----
info "MongoDB 探活..."
for i in $(seq 1 30); do
    kubectl exec -n ${NAMESPACE} deployment/mongodb -- mongosh --eval "db.adminCommand('ping')" --quiet 2>/dev/null && break
    [ $i -eq 30 ] && error "MongoDB 无响应"
    sleep 3
done
success "MongoDB 正常"
sleep 5

# ---- minio ----
step "初始化 MinIO"
kubectl wait --for=condition=complete job/minio-init-bucket -n ${NAMESPACE} --timeout=60s >/dev/null 2>&1 || warn "MinIO Job 未完成"
if [ -f "${IMAGE_FILE}" ]; then
    POD=$(kubectl get pod -l app=minio -n ${NAMESPACE} -o jsonpath='{.items[0].metadata.name}')
    cat "${IMAGE_FILE}" | kubectl exec -i -n ${NAMESPACE} "${POD}" -- sh -c "
        mc alias set local http://localhost:9000 ${MINIO_USER} ${MINIO_PASSWORD} --api s3v4 > /dev/null
        cat > /tmp/img.png
        mc cp /tmp/img.png local/${MINIO_BUCKET}/public/kube-nova.png
        rm /tmp/img.png
    " && success "图片上传成功" || warn "图片上传失败"
fi

# ---- mysql data ----
step "导入 MySQL 数据"
if ! kubectl exec -n ${NAMESPACE} deployment/mysql -- mysqladmin ping -uroot -p${MYSQL_PASSWORD} --silent 2>/dev/null; then
    kubectl exec -n ${NAMESPACE} deployment/mysql -- mysql -uroot -e "ALTER USER 'root'@'localhost' IDENTIFIED BY '${MYSQL_PASSWORD}'; FLUSH PRIVILEGES;" 2>/dev/null || true
fi
kubectl exec -i -n ${NAMESPACE} deployment/mysql -- mysql -uroot -p${MYSQL_PASSWORD} -e "CREATE DATABASE IF NOT EXISTS kube_nova DEFAULT CHARACTER SET utf8mb4;" 2>/dev/null
kubectl exec -i -n ${NAMESPACE} deployment/mysql -- mysql -uroot -p${MYSQL_PASSWORD} kube_nova < "${SQL_FILE}" && success "MySQL 导入成功" || error "MySQL 导入失败"

# ---- mongo data ----
if [ -d "${MONGO_DIR}" ]; then
    step "导入 MongoDB 模板数据"
    POD=$(kubectl get pod -l app=mongodb -n ${NAMESPACE} -o jsonpath='{.items[0].metadata.name}')
    URI="mongodb://root:${MONGO_PASSWORD}@localhost:27017/?authSource=admin"

    F="${MONGO_DIR}/devops_step_template.json"
    if [ -f "$F" ] && [ $(wc -c < "$F") -gt 3 ]; then
        kubectl exec -i -n ${NAMESPACE} "${POD}" -- mongoimport --uri="$URI" --db=kube_nova_devops --collection=devops_step_template --drop --jsonArray < "$F" 2>/dev/null && success "步骤模板导入成功" || warn "步骤模板导入失败"
    fi

    F="${MONGO_DIR}/devops_pipeline_template.json"
    if [ -f "$F" ] && [ $(wc -c < "$F") -gt 3 ]; then
        kubectl exec -i -n ${NAMESPACE} "${POD}" -- mongoimport --uri="$URI" --db=kube_nova_devops --collection=devops_pipeline_template --drop --jsonArray < "$F" 2>/dev/null && success "流水线模板导入成功" || warn "流水线模板导入失败"
    fi
fi

# ---- done ----
NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || echo "127.0.0.1")
JAEGER_PORT=$(kubectl get svc jaeger-ui -n ${NAMESPACE} -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "16686")

echo ""
echo "=========================================="
echo "  Kube-Nova 部署完成"
echo "=========================================="
echo "MySQL:    mysql.${NAMESPACE}.svc:3306"
echo "MongoDB:  mongodb.${NAMESPACE}.svc:27017"
echo "Redis:    redis.${NAMESPACE}.svc:6379"
echo "MinIO:    minio-service.${NAMESPACE}.svc:9000"
echo "Jaeger:   http://${NODE_IP}:${JAEGER_PORT}"
echo "=========================================="
