#!/bin/bash
# 数据迁移执行脚本：将项目数据统一迁移到 portal-rpc
# 使用方法: bash scripts/migration/02_migrate_data.sh
# 注意: 执行前必须备份数据，并在停服窗口内运行

set -e

MYSQL_HOST="172.16.1.61"
MYSQL_PORT="3307"
MYSQL_USER="root"
MYSQL_PASS="12345678"
MYSQL_DB="kube_nova"

MONGO_URI="mongodb://root:8VlZ2lvIsKBCYSE3@172.16.1.61:27017/kube_nova?authSource=admin"

echo "=========================================="
echo "项目数据迁移：Manager + DevOps -> Portal"
echo "=========================================="
echo ""
echo "警告: 此脚本会修改数据，请确保已备份！"
echo "按 Enter 继续，Ctrl+C 取消..."
read

# ==========================================
# 阶段 1: 备份
# ==========================================
echo ""
echo "=== 阶段 1: 数据备份 ==="

BACKUP_DIR="scripts/migration/backup_$(date +%Y%m%d_%H%M%S)"
mkdir -p "${BACKUP_DIR}"

echo "备份 Manager MySQL 数据..."
mysql -h${MYSQL_HOST} -P${MYSQL_PORT} -u${MYSQL_USER} -p${MYSQL_PASS} ${MYSQL_DB} -e "
CREATE TABLE IF NOT EXISTS ${BACKUP_DIR##*/}_onec_project AS SELECT * FROM onec_project;
" 2>/dev/null || echo "使用 mysqldump 备份..."

mysqldump -h${MYSQL_HOST} -P${MYSQL_PORT} -u${MYSQL_USER} -p${MYSQL_PASS} ${MYSQL_DB} \
  onec_project onec_project_admin onec_project_cluster onec_project_workspace \
  onec_project_application onec_project_version onec_project_audit_log \
  > "${BACKUP_DIR}/manager_backup.sql" 2>/dev/null

echo "备份 DevOps MongoDB 数据..."
mongosh "${MONGO_URI}" --quiet --eval "
  db.devops_project.aggregate([{\$match: {}}, {\$out: 'devops_project_backup'}]);
  db.devops_project_member.aggregate([{\$match: {}}, {\$out: 'devops_project_member_backup'}]);
  db.devops_project_channel_binding.aggregate([{\$match: {}}, {\$out: 'devops_project_channel_binding_backup'}]);
  db.devops_project_config.aggregate([{\$match: {}}, {\$out: 'devops_project_config_backup'}]);
  db.devops_project_maven_config.aggregate([{\$match: {}}, {\$out: 'devops_project_maven_config_backup'}]);
  db.devops_system.aggregate([{\$match: {}}, {\$out: 'devops_system_backup'}]);
  db.devops_credential.aggregate([{\$match: {}}, {\$out: 'devops_credential_backup'}]);
  print('MongoDB 备份完成');
"

echo "备份完成: ${BACKUP_DIR}"

# ==========================================
# 阶段 2: 迁移 Manager 项目到 Portal
# ==========================================
echo ""
echo "=== 阶段 2: 迁移 Manager 项目 ==="

# portal-rpc 使用同一个 MySQL 数据库，onec_project 表已存在
# 只需要确认 portal-rpc 的 model 指向正确的表即可
# 如果 portal-rpc 使用独立数据库，需要执行以下操作:

echo "Manager onec_project 表已在 portal-rpc 共享的 MySQL 中"
echo "无需额外迁移，portal-rpc 直接读写同一张表"

# 验证表结构一致
echo "验证 onec_project 表结构..."
mysql -h${MYSQL_HOST} -P${MYSQL_PORT} -u${MYSQL_USER} -p${MYSQL_PASS} ${MYSQL_DB} -e "
DESC onec_project;
" 2>/dev/null

# ==========================================
# 阶段 3: 为 DevOps 项目建立 portalProjectUuid 映射
# ==========================================
echo ""
echo "=== 阶段 3: 建立 DevOps 项目映射 ==="

# 3.1 同名覆盖：Manager 和 DevOps 都有且 name 相同的项目
echo "处理同名项目（Manager 为准）..."
mysql -h${MYSQL_HOST} -P${MYSQL_PORT} -u${MYSQL_USER} -p${MYSQL_PASS} ${MYSQL_DB} -sN -e "
SELECT p.name, p.uuid
FROM onec_project p
WHERE p.is_deleted = 0
ORDER BY p.id;
" 2>/dev/null | while read line; do
  name=$(echo "$line" | awk '{print $1}')
  uuid=$(echo "$line" | awk '{print $2}')
  
  mongosh "${MONGO_URI}" --quiet --eval "
    var result = db.devops_project.updateMany(
      {name: '${name}', isDeleted: false, portalProjectUuid: {\$exists: false}},
      {\$set: {portalProjectUuid: '${uuid}'}}
    );
    if (result.modifiedCount > 0) {
      print('同名覆盖: name=${name} -> portalProjectUuid=${uuid}, 更新了 ' + result.modifiedCount + ' 条');
    }
  "
done

# 3.2 异名增量：DevOps 独有的项目，生成新 UUID 并创建 portal 记录
echo ""
echo "处理 DevOps 独有项目..."
mongosh "${MONGO_URI}" --quiet --eval '
db.devops_project.find({
  isDeleted: false,
  portalProjectUuid: {$exists: false}
}, {name: 1, code: 1, description: 1, createdBy: 1}).forEach(doc => {
  print("DevOps独有: name=" + doc.name + ", code=" + doc.code);
});
' | while read line; do
  if echo "$line" | grep -q "DevOps独有:"; then
    name=$(echo "$line" | sed 's/DevOps独有: name=//' | cut -d',' -f1)
    code=$(echo "$line" | sed 's/.*code=//')
    
    # 生成新 UUID
    new_uuid=$(uuidgen | tr '[:upper:]' '[:lower:]')
    
    echo "  创建 portal 项目: name=$name, uuid=$new_uuid"
    
    # 在 portal MySQL 中创建项目
    mysql -h${MYSQL_HOST} -P${MYSQL_PORT} -u${MYSQL_USER} -p${MYSQL_PASS} ${MYSQL_DB} -e "
      INSERT INTO onec_project (name, uuid, is_system, description, created_by, updated_by)
      VALUES ('${name}', '${new_uuid}', 0, '从 DevOps 迁移', 'migration', 'migration');
    " 2>/dev/null
    
    # 更新 DevOps 项目
    mongosh "${MONGO_URI}" --quiet --eval "
      db.devops_project.updateOne(
        {code: '${code}', isDeleted: false},
        {\$set: {portalProjectUuid: '${new_uuid}'}}
      );
      print('  已关联: code=${code} -> portalProjectUuid=${new_uuid}');
    "
  fi
done

# ==========================================
# 阶段 4: 清理孤立数据
# ==========================================
echo ""
echo "=== 阶段 4: 清理孤立数据 ==="

echo "检查 Manager 侧孤立数据..."
mysql -h${MYSQL_HOST} -P${MYSQL_PORT} -u${MYSQL_USER} -p${MYSQL_PASS} ${MYSQL_DB} -e "
-- 查找 project_cluster 中 project_id 无效的记录
SELECT 'project_cluster 孤立记录' as type, COUNT(*) as count
FROM onec_project_cluster pc
LEFT JOIN onec_project p ON pc.project_id = p.id
WHERE p.id IS NULL AND pc.is_deleted = 0;

-- 查找 project_admin 中 project_id 无效的记录
SELECT 'project_admin 孤立记录' as type, COUNT(*) as count
FROM onec_project_admin pa
LEFT JOIN onec_project p ON pa.project_id = p.id
WHERE p.id IS NULL AND pa.is_deleted = 0;

-- 查找 project_audit_log 中 project_id 无效的记录
SELECT 'project_audit_log 孤立记录' as type, COUNT(*) as count
FROM onec_project_audit_log pal
LEFT JOIN onec_project p ON pal.project_id = p.id
WHERE p.id IS NULL AND pal.is_deleted = 0;
" 2>/dev/null

echo ""
echo "检查 DevOps 侧孤立数据..."
mongosh "${MONGO_URI}" --quiet --eval '
var projectIds = db.devops_project.distinct("_id", {isDeleted: false}).map(id => id.toString());

// 检查 credential
var orphanCreds = db.devops_credential.countDocuments({
  scope: "project",
  isDeleted: false,
  projectId: {$nin: projectIds}
});
print("devops_credential 孤立记录: " + orphanCreds);

// 检查 system
var orphanSystems = db.devops_system.countDocuments({
  isDeleted: false,
  projectId: {$nin: projectIds}
});
print("devops_system 孤立记录: " + orphanSystems);

// 检查 member
var orphanMembers = db.devops_project_member.countDocuments({
  isDeleted: false,
  projectId: {$nin: projectIds}
});
print("devops_project_member 孤立记录: " + orphanMembers);

// 检查 channel_binding
var orphanBindings = db.devops_project_channel_binding.countDocuments({
  isDeleted: false,
  projectId: {$nin: projectIds}
});
print("devops_project_channel_binding 孤立记录: " + orphanBindings);
'

echo ""
echo "=========================================="
echo "迁移完成"
echo "备份目录: ${BACKUP_DIR}"
echo "=========================================="
