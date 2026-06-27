#!/bin/bash
# 数据迁移分析脚本：对比 Manager 和 DevOps 的项目数据
# 使用方法: bash scripts/migration/01_analyze_data.sh

set -e

MYSQL_HOST="172.16.1.61"
MYSQL_PORT="3307"
MYSQL_USER="root"
MYSQL_PASS="12345678"
MYSQL_DB="kube_nova"

MONGO_URI="mongodb://root:8VlZ2lvIsKBCYSE3@172.16.1.61:27017/kube_nova?authSource=admin"

echo "=========================================="
echo "数据迁移分析：Manager vs DevOps 项目对比"
echo "=========================================="

echo ""
echo "=== 1. Manager onec_project 表数据 ==="
mysql -h${MYSQL_HOST} -P${MYSQL_PORT} -u${MYSQL_USER} -p${MYSQL_PASS} ${MYSQL_DB} -e "
SELECT id, name, uuid, is_system, is_deleted, created_at
FROM onec_project
ORDER BY id;
" 2>/dev/null

echo ""
echo "=== 2. DevOps devops_project 集合数据 ==="
mongosh "${MONGO_URI}" --quiet --eval '
db.devops_project.find({}, {
  _id: 1, name: 1, code: 1, description: 1, status: 1, isDeleted: 1, createAt: 1
}).sort({createAt: 1}).forEach(doc => {
  printjson(doc);
});
'

echo ""
echo "=== 3. 同名项目对比 (name 相同) ==="
echo "通过 name 字段匹配的项目:"
mysql -h${MYSQL_HOST} -P${MYSQL_PORT} -u${MYSQL_USER} -p${MYSQL_PASS} ${MYSQL_DB} -sN -e "
SELECT p.id, p.name, p.uuid
FROM onec_project p
WHERE p.is_deleted = 0
ORDER BY p.id;
" 2>/dev/null | while read line; do
  id=$(echo "$line" | awk '{print $1}')
  name=$(echo "$line" | awk '{print $2}')
  uuid=$(echo "$line" | awk '{print $3}')
  
  mongo_count=$(mongosh "${MONGO_URI}" --quiet --eval "
    db.devops_project.countDocuments({name: '${name}', isDeleted: false});
  ")
  
  if [ "$mongo_count" -gt 0 ]; then
    echo "  [同名] Manager(id=$id, name=$name, uuid=$uuid) <-> DevOps(name=$name)"
  fi
done

echo ""
echo "=== 4. Manager 独有项目 ==="
mysql -h${MYSQL_HOST} -P${MYSQL_PORT} -u${MYSQL_USER} -p${MYSQL_PASS} ${MYSQL_DB} -sN -e "
SELECT p.id, p.name, p.uuid
FROM onec_project p
WHERE p.is_deleted = 0
ORDER BY p.id;
" 2>/dev/null | while read line; do
  id=$(echo "$line" | awk '{print $1}')
  name=$(echo "$line" | awk '{print $2}')
  uuid=$(echo "$line" | awk '{print $3}')
  
  mongo_count=$(mongosh "${MONGO_URI}" --quiet --eval "
    db.devops_project.countDocuments({name: '${name}', isDeleted: false});
  ")
  
  if [ "$mongo_count" -eq 0 ]; then
    echo "  [Manager独有] id=$id, name=$name, uuid=$uuid"
  fi
done

echo ""
echo "=== 5. DevOps 独有项目 ==="
mongosh "${MONGO_URI}" --quiet --eval '
db.devops_project.find({isDeleted: false}, {name: 1, code: 1}).forEach(doc => {
  print(doc.name + "|" + doc.code);
});
' | while read line; do
  name=$(echo "$line" | cut -d'|' -f1)
  code=$(echo "$line" | cut -d'|' -f2)
  
  mysql_count=$(mysql -h${MYSQL_HOST} -P${MYSQL_PORT} -u${MYSQL_USER} -p${MYSQL_PASS} ${MYSQL_DB} -sN -e "
    SELECT COUNT(*) FROM onec_project WHERE name = '${name}' AND is_deleted = 0;
  " 2>/dev/null)
  
  if [ "$mysql_count" -eq 0 ]; then
    echo "  [DevOps独有] name=$name, code=$code"
  fi
done

echo ""
echo "=== 6. 孤立数据检查 ==="
echo "Manager 侧 - project_cluster 中 project_id 无效的:"
mysql -h${MYSQL_HOST} -P${MYSQL_PORT} -u${MYSQL_USER} -p${MYSQL_PASS} ${MYSQL_DB} -e "
SELECT pc.id, pc.project_id, pc.cluster_uuid
FROM onec_project_cluster pc
LEFT JOIN onec_project p ON pc.project_id = p.id
WHERE p.id IS NULL AND pc.is_deleted = 0;
" 2>/dev/null

echo ""
echo "DevOps 侧 - devops_credential 中 projectId 无效的:"
mongosh "${MONGO_URI}" --quiet --eval '
var projectIds = db.devops_project.distinct("_id");
db.devops_credential.find({
  scope: "project",
  isDeleted: false,
  projectId: {$nin: projectIds.map(id => id.toString())}
}, {name: 1, projectId: 1}).forEach(doc => {
  printjson(doc);
});
'

echo ""
echo "=========================================="
echo "分析完成"
echo "=========================================="
