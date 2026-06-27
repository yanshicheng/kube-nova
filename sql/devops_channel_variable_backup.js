// 渠道变量重设前备份脚本。
// 用法：mongosh "<mongo-url>/<db>" sql/devops_channel_variable_backup.js

const dbName = process.env.DEVOPS_MONGO_DB || db.getName();
const backupSuffix =
  process.env.DEVOPS_BACKUP_SUFFIX ||
  new Date().toISOString().replace(/[-:T.Z]/g, '').slice(0, 14);
const targetDb = db.getSiblingDB(dbName);

const collections = [
  'devops_channel_group',
  'devops_channel_type',
  'devops_channel',
  'devops_step_channel_param',
  'devops_step_template',
  'devops_pipeline_template',
  'devops_project_channel_binding'
];

collections.forEach((name) => {
  const backupName = `${name}_bak_${backupSuffix}`;
  const exists = targetDb.getCollectionNames().includes(name);
  if (!exists) {
    print(`skip ${name}, collection not exists`);
    return;
  }
  targetDb.getCollection(name).aggregate([{ $match: {} }, { $out: backupName }]);
  print(`backup ${name} -> ${backupName}`);
});
