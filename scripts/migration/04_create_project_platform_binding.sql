-- 项目-平台绑定表
-- 控制项目可以访问哪些平台
CREATE TABLE IF NOT EXISTS `project_platform_binding` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `project_id` BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  `platform_id` BIGINT UNSIGNED NOT NULL COMMENT '平台ID',
  `created_by` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '记录创建人',
  `updated_by` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '记录最后更新人',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '记录创建时间',
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '记录更新时间',
  `is_deleted` TINYINT(1) NOT NULL DEFAULT '0' COMMENT '是否已删除，软删除标记',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_platform` (`project_id`, `platform_id`),
  KEY `idx_project_id` (`project_id`),
  KEY `idx_platform_id` (`platform_id`),
  KEY `idx_is_deleted` (`is_deleted`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='项目-平台绑定表，控制项目可访问的平台';

-- 项目成员绑定表
CREATE TABLE IF NOT EXISTS `project_member_binding` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `project_id` BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  `role` VARCHAR(32) NOT NULL DEFAULT 'member' COMMENT '项目角色：owner/admin/member',
  `created_by` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '记录创建人',
  `updated_by` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '记录最后更新人',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '记录创建时间',
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '记录更新时间',
  `is_deleted` TINYINT(1) NOT NULL DEFAULT '0' COMMENT '是否已删除，软删除标记',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_user` (`project_id`, `user_id`),
  KEY `idx_project_id` (`project_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_is_deleted` (`is_deleted`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='项目成员绑定表';

-- 项目成员平台角色表
CREATE TABLE IF NOT EXISTS `project_member_platform_role` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `project_id` BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  `platform_id` BIGINT UNSIGNED NOT NULL COMMENT '平台ID',
  `role` VARCHAR(32) NOT NULL DEFAULT 'member' COMMENT '项目平台角色：owner/admin/member',
  `created_by` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '记录创建人',
  `updated_by` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '记录最后更新人',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '记录创建时间',
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '记录更新时间',
  `is_deleted` TINYINT(1) NOT NULL DEFAULT '0' COMMENT '是否已删除，软删除标记',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_user_platform` (`project_id`, `user_id`, `platform_id`),
  KEY `idx_project_id` (`project_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_platform_id` (`platform_id`),
  KEY `idx_is_deleted` (`is_deleted`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='项目成员平台角色表';

-- 项目平台投影同步任务表
CREATE TABLE IF NOT EXISTS `project_platform_sync_task` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `project_id` BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  `portal_project_uuid` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '门户项目UUID',
  `platform_code` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '平台编码',
  `action` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '同步动作：project_info/project_delete/project_members',
  `payload` LONGTEXT NOT NULL COMMENT '同步快照JSON',
  `status` VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT '同步状态：pending/success/failed',
  `retry_count` INT NOT NULL DEFAULT '0' COMMENT '重试次数',
  `last_error` VARCHAR(500) NOT NULL DEFAULT '' COMMENT '最后一次错误信息',
  `last_synced_at` TIMESTAMP NOT NULL DEFAULT '1970-01-01 00:00:00' COMMENT '最后同步成功时间',
  `created_by` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '记录创建人',
  `updated_by` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '记录最后更新人',
  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '记录创建时间',
  `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '记录更新时间',
  `is_deleted` TINYINT(1) NOT NULL DEFAULT '0' COMMENT '是否已删除，软删除标记',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_platform_action_project` (`platform_code`, `action`, `portal_project_uuid`),
  KEY `idx_status_retry` (`status`, `retry_count`),
  KEY `idx_project_id` (`project_id`),
  KEY `idx_is_deleted` (`is_deleted`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='项目平台投影同步任务表';

-- 回填项目平台绑定：默认平台、容器平台、门户平台
INSERT INTO `project_platform_binding` (
  `project_id`,
  `platform_id`,
  `created_by`,
  `updated_by`,
  `is_deleted`
)
SELECT DISTINCT
  p.`id`,
  sp.`id`,
  'migration',
  'migration',
  0
FROM `onec_project` p
INNER JOIN `sys_platform` sp
  ON sp.`is_enable` = 1
  AND sp.`is_deleted` = 0
  AND (sp.`is_default` = 1 OR sp.`platform_code` IN ('container', 'portal'))
WHERE p.`is_deleted` = 0
ON DUPLICATE KEY UPDATE
  `updated_by` = VALUES(`updated_by`),
  `updated_at` = NOW(),
  `is_deleted` = 0;

-- 回填 DevOps 迁移项目的平台绑定
INSERT INTO `project_platform_binding` (
  `project_id`,
  `platform_id`,
  `created_by`,
  `updated_by`,
  `is_deleted`
)
SELECT DISTINCT
  p.`id`,
  sp.`id`,
  'migration',
  'migration',
  0
FROM `onec_project` p
INNER JOIN `sys_platform` sp
  ON sp.`platform_code` = 'devops'
  AND sp.`is_enable` = 1
  AND sp.`is_deleted` = 0
WHERE p.`is_deleted` = 0
  AND p.`description` = '从 DevOps 迁移'
ON DUPLICATE KEY UPDATE
  `updated_by` = VALUES(`updated_by`),
  `updated_at` = NOW(),
  `is_deleted` = 0;

-- 回填原项目管理员为项目成员
INSERT INTO `project_member_binding` (
  `project_id`,
  `user_id`,
  `role`,
  `created_by`,
  `updated_by`,
  `is_deleted`
)
SELECT DISTINCT
  pa.`project_id`,
  pa.`user_id`,
  'admin',
  'migration',
  'migration',
  0
FROM `onec_project_admin` pa
INNER JOIN `onec_project` p
  ON p.`id` = pa.`project_id`
  AND p.`is_deleted` = 0
INNER JOIN `sys_user` su
  ON su.`id` = pa.`user_id`
  AND su.`is_deleted` = 0
WHERE pa.`is_deleted` = 0
ON DUPLICATE KEY UPDATE
  `role` = VALUES(`role`),
  `updated_by` = VALUES(`updated_by`),
  `updated_at` = NOW(),
  `is_deleted` = 0;

-- 回填超级管理员为所有项目 owner
INSERT INTO `project_member_binding` (
  `project_id`,
  `user_id`,
  `role`,
  `created_by`,
  `updated_by`,
  `is_deleted`
)
SELECT DISTINCT
  p.`id`,
  su.`id`,
  'owner',
  'migration',
  'migration',
  0
FROM `onec_project` p
INNER JOIN `sys_user` su
  ON su.`is_deleted` = 0
LEFT JOIN `sys_user_role` sur
  ON sur.`user_id` = su.`id`
  AND sur.`is_deleted` = 0
LEFT JOIN `sys_role` sr
  ON sr.`id` = sur.`role_id`
  AND sr.`is_deleted` = 0
WHERE p.`is_deleted` = 0
  AND (su.`username` = 'super_admin' OR sr.`code` IN ('super_admin', 'SUPER_ADMIN', 'R_SUPER'))
ON DUPLICATE KEY UPDATE
  `role` = VALUES(`role`),
  `updated_by` = VALUES(`updated_by`),
  `updated_at` = NOW(),
  `is_deleted` = 0;

-- 回填项目成员在已绑定平台下的角色
INSERT INTO `project_member_platform_role` (
  `project_id`,
  `user_id`,
  `platform_id`,
  `role`,
  `created_by`,
  `updated_by`,
  `is_deleted`
)
SELECT DISTINCT
  pmb.`project_id`,
  pmb.`user_id`,
  ppb.`platform_id`,
  pmb.`role`,
  'migration',
  'migration',
  0
FROM `project_member_binding` pmb
INNER JOIN `project_platform_binding` ppb
  ON ppb.`project_id` = pmb.`project_id`
  AND ppb.`is_deleted` = 0
WHERE pmb.`is_deleted` = 0
ON DUPLICATE KEY UPDATE
  `role` = VALUES(`role`),
  `updated_by` = VALUES(`updated_by`),
  `updated_at` = NOW(),
  `is_deleted` = 0;
