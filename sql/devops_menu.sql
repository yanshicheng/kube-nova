-- DevOps 平台动态菜单与 API 权限初始化
-- 平台 ID 固定为 3

SET NAMES utf8mb4;

START TRANSACTION;

-- 旧版 DevOps 总目录停用，平台内直接展示业务一级目录
UPDATE `sys_menu`
SET `is_deleted` = 1, `update_by` = 'system'
WHERE `platform_id` = 3 AND `name` = 'DevOps';

-- 复用平台已有仪表盘主菜单，避免重复创建一级仪表盘
UPDATE `sys_menu`
SET `is_deleted` = 1, `is_enable` = 0, `is_menu` = 0, `update_by` = 'system'
WHERE `platform_id` = 3 AND `name` = 'DevOpsDashboard';

SET @dashboard_menu_id := (
  SELECT `id` FROM `sys_menu`
  WHERE `platform_id` = 3
    AND `parent_id` = 0
    AND `is_deleted` = 0
    AND (`name` = 'DDashboard' OR `title` = '仪表盘' OR `path` = '/dashboard')
  ORDER BY CASE
    WHEN `name` = 'DDashboard' THEN 0
    WHEN `title` = '仪表盘' THEN 1
    ELSE 2
  END, `id`
  LIMIT 1
);

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @dashboard_menu_id, 3, 2, 'DevOpsDashboardCockpit', 'cockpit', '/devops/dashboard/cockpit', '', '',
  '驾驶舱', 'ri:dashboard-line', 1, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  0, 3, 1, 'DevOpsQualityManagement', '/quality', '/index/index', '', '',
  '质量管理', 'ri:shield-check-line', 6, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

SET @quality_menu_id := (
  SELECT `id` FROM `sys_menu`
  WHERE `platform_id` = 3 AND `name` = 'DevOpsQualityManagement' AND `is_deleted` = 0
  LIMIT 1
);

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @quality_menu_id, 3, 2, 'DevOpsQualityStaticCode', 'static-code', '/devops/quality/static-code', '', '',
  '静态代码', 'ri:code-box-line', 2, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @quality_menu_id, 3, 2, 'DevOpsQualityImageVulnerability', 'image-vulnerability', '/devops/quality/image-vulnerability', '', '',
  '镜像漏洞', 'ri:shield-keyhole-line', 3, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @dashboard_menu_id, 3, 2, 'DevOpsDashboardProject', 'project', '/devops/dashboard/project', '', '',
  '项目大盘', 'ri:bar-chart-box-line', 2, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  0, 3, 1, 'DevOpsProjectManagement', '/project', '/index/index', '', '',
  '项目管理', 'ri:folder-settings-line', 1, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

SET @project_menu_id := (
  SELECT `id` FROM `sys_menu`
  WHERE `platform_id` = 3 AND `name` = 'DevOpsProjectManagement' AND `is_deleted` = 0
  LIMIT 1
);

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @project_menu_id, 3, 2, 'DevOpsProjectCenter', 'center', '/devops/project/center', '', '',
  '项目中心', 'ri:folder-3-line', 1, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @project_menu_id, 3, 2, 'DevOpsProjectSystem', 'system', '/devops/project/system', '', '',
  '应用管理', 'ri:apps-2-line', 2, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @project_menu_id, 3, 2, 'DevOpsProjectChannelBinding', 'channel', '/devops/project/channel', '', '',
  '项目绑定渠道', 'ri:links-line', 3, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  0, 3, 1, 'DevOpsChannelManagement', '/channel', '/index/index', '', '',
  '渠道管理', 'ri:git-branch-line', 3, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

SET @channel_menu_id := (
  SELECT `id` FROM `sys_menu`
  WHERE `platform_id` = 3 AND `name` = 'DevOpsChannelManagement' AND `is_deleted` = 0
  LIMIT 1
);

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @channel_menu_id, 3, 2, 'DevOpsChannelGroup', 'group', '/devops/channel/group', '', '',
  '渠道分组', 'ri:folder-2-line', 1, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @channel_menu_id, 3, 2, 'DevOpsChannelCenter', 'center', '/devops/channel/center', '', '',
  '渠道中心', 'ri:apps-2-line', 2, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @channel_menu_id, 3, 2, 'DevOpsChannelHost', 'host', '/devops/channel/host', '', '',
  '主机资产', 'ri:server-line', 3, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  0, 3, 1, 'DevOpsSystemSettings', '/settings', '/index/index', '', '',
  '系统设置', 'ri:settings-3-line', 4, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

SET @settings_menu_id := (
  SELECT `id` FROM `sys_menu`
  WHERE `platform_id` = 3 AND `name` = 'DevOpsSystemSettings' AND `is_deleted` = 0
  LIMIT 1
);

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @settings_menu_id, 3, 2, 'DevOpsCredentialManagement', 'credential', '/devops/settings/credential', '', '',
  '凭证管理', 'ri:key-2-line', 1, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @settings_menu_id, 3, 2, 'DevOpsJenkinsCredential', 'jenkins-credential', '/devops/settings/jenkins-credential', '', '',
  'Jenkins 凭证', 'ri:key-2-line', 2, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

UPDATE `sys_menu`
SET `is_enable` = 0,
    `is_menu` = 0,
    `is_deleted` = 1,
    `update_by` = 'system'
WHERE `platform_id` = 3
  AND `name` = 'DevOpsTektonSecret';

UPDATE `sys_role_menu` rm
JOIN `sys_menu` m ON m.`id` = rm.`menu_id`
SET rm.`is_deleted` = 1
WHERE m.`platform_id` = 3
  AND m.`name` = 'DevOpsTektonSecret';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @settings_menu_id, 3, 2, 'DevOpsTektonResource', 'tekton-resource', '/devops/settings/tekton-resource', '', '',
  'Tekton 资源管理', 'ri:database-2-line', 3, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @settings_menu_id, 3, 2, 'DevOpsTektonRuntimeResource', 'tekton-runtime-resource', '/devops/settings/tekton-runtime-resource', '', '',
  'Tekton 资源查询', 'ri:search-eye-line', 4, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @settings_menu_id, 3, 2, 'DevOpsConfigCenter', 'config-center', '/devops/settings/config-center', '', '',
  '配置中心', 'ri:settings-4-line', 5, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @settings_menu_id, 3, 2, 'DevOpsConfigType', 'config-type', '/devops/settings/config-type', '', '',
  '配置类型', 'ri:list-settings-line', 6, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @settings_menu_id, 3, 2, 'DevOpsProjectChannelSettings', 'channel', '/devops/settings/channel', '', '',
  '渠道管理', 'ri:git-branch-line', 7, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  0, 3, 1, 'DevOpsPipelineConfig', '/pipeline', '/index/index', '', '',
  '流水线配置', 'ri:flow-chart', 2, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

SET @pipeline_menu_id := (
  SELECT `id` FROM `sys_menu`
  WHERE `platform_id` = 3 AND `name` = 'DevOpsPipelineConfig' AND `is_deleted` = 0
  LIMIT 1
);

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @pipeline_menu_id, 3, 2, 'DevOpsTektonPolicy', 'tekton-policy', '/devops/pipeline/tekton-policy', '', '',
  'Tekton 策略', 'ri:git-branch-line', 7, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @pipeline_menu_id, 3, 2, 'DevOpsJenkinsStep', 'jenkins-step', '/devops/pipeline/jenkins-step', '', '',
  'Jenkins 步骤', 'simple-icons:jenkins', 1, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @pipeline_menu_id, 3, 2, 'DevOpsTektonStep', 'tekton-step', '/devops/pipeline/tekton-step', '', '',
  'Tekton 步骤', 'ri:flow-chart', 2, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @pipeline_menu_id, 3, 2, 'DevOpsPipelineEnvironment', 'environment', '/devops/pipeline/environment', '', '',
  '流水线环境', 'ri:server-line', 5, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

UPDATE `sys_menu` old_menu
LEFT JOIN `sys_menu` new_menu
  ON new_menu.`platform_id` = 3
  AND new_menu.`name` = 'DevOpsJenkinsTemplateLibrary'
SET
  old_menu.`name` = 'DevOpsJenkinsTemplateLibrary',
  old_menu.`path` = 'jenkins-template',
  old_menu.`component` = '/devops/pipeline/template',
  old_menu.`title` = 'Jenkins 模板库',
  old_menu.`icon` = 'simple-icons:jenkins',
  old_menu.`sort` = 3,
  old_menu.`is_enable` = 1,
  old_menu.`is_menu` = 1,
  old_menu.`is_hide` = 0,
  old_menu.`is_deleted` = 0,
  old_menu.`update_by` = 'system'
WHERE old_menu.`platform_id` = 3
  AND old_menu.`name` = 'DevOpsPipelineTemplate'
  AND new_menu.`id` IS NULL;

UPDATE `sys_menu`
SET `is_deleted` = 1, `is_menu` = 0, `is_enable` = 0, `update_by` = 'system'
WHERE `platform_id` = 3 AND `name` = 'DevOpsPipelineTemplate';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @pipeline_menu_id, 3, 2, 'DevOpsJenkinsTemplateLibrary', 'jenkins-template', '/devops/pipeline/template', '', '',
  'Jenkins 模板库', 'simple-icons:jenkins', 3, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @pipeline_menu_id, 3, 2, 'DevOpsTektonTemplateLibrary', 'tekton-template', '/devops/pipeline/tekton-template', '', '',
  'Tekton 模板库', 'simple-icons:tekton', 4, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_hide` = VALUES(`is_hide`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @pipeline_menu_id, 3, 2, 'DevOpsStepCategory', 'category', '/devops/pipeline/category', '', '',
  '步骤分类', 'ri:folder-2-line', 6, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  0, 3, 1, 'DevOpsPipelineManagement', '/pipeline-management', '/index/index', '', '',
  '流水线管理', 'ri:node-tree', 5, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

SET @pipeline_manage_menu_id := (
  SELECT `id` FROM `sys_menu`
  WHERE `platform_id` = 3 AND `name` = 'DevOpsPipelineManagement' AND `is_deleted` = 0
  LIMIT 1
);

UPDATE `sys_menu`
SET
  `parent_id` = @pipeline_manage_menu_id,
  `path` = 'application',
  `component` = '/devops/project/system',
  `title` = '应用管理',
  `icon` = 'ri:apps-2-line',
  `sort` = 1,
  `is_enable` = 1,
  `is_menu` = 1,
  `is_deleted` = 0,
  `update_by` = 'system'
WHERE `platform_id` = 3
  AND `name` = 'DevOpsProjectSystem'
  AND @pipeline_manage_menu_id IS NOT NULL;

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @pipeline_manage_menu_id, 3, 2, 'DevOpsPipelineInstance', 'pipeline', '/devops/pipeline/instance', '', '',
  '流水线', 'ri:node-tree', 2, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

SET @quality_menu_id := (
  SELECT `id` FROM `sys_menu`
  WHERE `platform_id` = 3 AND `name` = 'DevOpsQualityManagement' AND `is_deleted` = 0
  LIMIT 1
);

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @quality_menu_id, 3, 2, 'DevOpsQualityDashboard', 'dashboard', '/devops/quality/dashboard', '', '',
  '质量看板', 'ri:bar-chart-grouped-line', 1, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @quality_menu_id, 3, 2, 'DevOpsQualityReport', 'report', '/devops/quality/report', '', '',
  '扫描报告', 'ri:file-list-3-line', 4, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @quality_menu_id, 3, 2, 'DevOpsQualityUploadBatch', 'upload-batch', '/devops/quality/upload-batch', '', '',
  '上传批次', 'ri:archive-line', 5, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_menu` (
  `parent_id`, `platform_id`, `menu_type`, `name`, `path`, `component`, `redirect`, `label`,
  `title`, `icon`, `sort`, `link`, `is_enable`, `is_menu`, `keep_alive`, `is_hide`,
  `is_iframe`, `is_hide_tab`, `show_badge`, `show_text_badge`, `is_first_level`,
  `fixed_tab`, `is_full_page`, `active_path`, `roles`, `auth_name`, `auth_label`,
  `auth_icon`, `auth_sort`, `status`, `is_deleted`, `create_by`, `update_by`
) VALUES (
  @quality_menu_id, 3, 2, 'DevOpsQualityIssue', 'issue', '/devops/quality/issue', '', '',
  '问题管理', 'ri:bug-line', 6, '', 1, 1, 1, 0,
  0, 0, 0, '', 0, 0, 0, '', NULL, '', '',
  '', 0, 1, 0, 'system', 'system'
) ON DUPLICATE KEY UPDATE
  `parent_id` = VALUES(`parent_id`),
  `menu_type` = VALUES(`menu_type`),
  `path` = VALUES(`path`),
  `component` = VALUES(`component`),
  `title` = VALUES(`title`),
  `icon` = VALUES(`icon`),
  `sort` = VALUES(`sort`),
  `is_enable` = VALUES(`is_enable`),
  `is_menu` = VALUES(`is_menu`),
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_api` (`parent_id`, `name`, `path`, `method`, `is_permission`, `created_by`, `updated_by`, `is_deleted`)
SELECT 0, 'DevOps模块', '', '', 0, 'system', 'system', 0
WHERE NOT EXISTS (
  SELECT 1 FROM `sys_api` WHERE `parent_id` = 0 AND `name` = 'DevOps模块' AND `is_deleted` = 0
);

SET @devops_api_id := (
  SELECT `id` FROM `sys_api`
  WHERE `parent_id` = 0 AND `name` = 'DevOps模块' AND `is_deleted` = 0
  LIMIT 1
);

INSERT INTO `sys_api` (`parent_id`, `name`, `path`, `method`, `is_permission`, `created_by`, `updated_by`, `is_deleted`)
SELECT @devops_api_id, 'DevOps模块所有权限', '/devops/v1/*', '*', 1, 'system', 'system', 0
WHERE @devops_api_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM `sys_api`
    WHERE `parent_id` = @devops_api_id AND `path` = '/devops/v1/*' AND `method` = '*' AND `is_deleted` = 0
  );

INSERT INTO `sys_api` (`parent_id`, `name`, `path`, `method`, `is_permission`, `created_by`, `updated_by`, `is_deleted`)
SELECT @devops_api_id, 'DevOps模块查询权限', '/devops/v1/*', 'GET', 1, 'system', 'system', 0
WHERE @devops_api_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM `sys_api`
    WHERE `parent_id` = @devops_api_id AND `path` = '/devops/v1/*' AND `method` = 'GET' AND `is_deleted` = 0
  );

INSERT INTO `sys_api` (`parent_id`, `name`, `path`, `method`, `is_permission`, `created_by`, `updated_by`, `is_deleted`)
SELECT @devops_api_id, 'DevOps质量管理所有权限', '/devops/v1/quality/*', '*', 1, 'system', 'system', 0
WHERE @devops_api_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM `sys_api`
    WHERE `parent_id` = @devops_api_id AND `path` = '/devops/v1/quality/*' AND `method` = '*' AND `is_deleted` = 0
  );

INSERT INTO `sys_api` (`parent_id`, `name`, `path`, `method`, `is_permission`, `created_by`, `updated_by`, `is_deleted`)
SELECT @devops_api_id, 'DevOps质量管理查询权限', '/devops/v1/quality/*', 'GET', 1, 'system', 'system', 0
WHERE @devops_api_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM `sys_api`
    WHERE `parent_id` = @devops_api_id AND `path` = '/devops/v1/quality/*' AND `method` = 'GET' AND `is_deleted` = 0
  );

INSERT INTO `sys_api` (`parent_id`, `name`, `path`, `method`, `is_permission`, `created_by`, `updated_by`, `is_deleted`)
SELECT @devops_api_id, '上传质量报告', '/devops/v1/quality/report/upload', 'POST', 1, 'system', 'system', 0
WHERE @devops_api_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM `sys_api`
    WHERE `parent_id` = @devops_api_id AND `path` = '/devops/v1/quality/report/upload' AND `method` = 'POST' AND `is_deleted` = 0
  );

INSERT INTO `sys_api` (`parent_id`, `name`, `path`, `method`, `is_permission`, `created_by`, `updated_by`, `is_deleted`)
SELECT @devops_api_id, '下载质量报告', '/devops/v1/quality/report/*/download', 'GET', 1, 'system', 'system', 0
WHERE @devops_api_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM `sys_api`
    WHERE `parent_id` = @devops_api_id AND `path` = '/devops/v1/quality/report/*/download' AND `method` = 'GET' AND `is_deleted` = 0
  );

INSERT INTO `sys_api` (`parent_id`, `name`, `path`, `method`, `is_permission`, `created_by`, `updated_by`, `is_deleted`)
SELECT @devops_api_id, '预览质量报告', '/devops/v1/quality/report/*/preview', 'GET', 1, 'system', 'system', 0
WHERE @devops_api_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM `sys_api`
    WHERE `parent_id` = @devops_api_id AND `path` = '/devops/v1/quality/report/*/preview' AND `method` = 'GET' AND `is_deleted` = 0
  );

INSERT INTO `sys_api` (`parent_id`, `name`, `path`, `method`, `is_permission`, `created_by`, `updated_by`, `is_deleted`)
SELECT @devops_api_id, '下载质量上传批次原件', '/devops/v1/quality/report/upload-batch/*/download', 'GET', 1, 'system', 'system', 0
WHERE @devops_api_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM `sys_api`
    WHERE `parent_id` = @devops_api_id AND `path` = '/devops/v1/quality/report/upload-batch/*/download' AND `method` = 'GET' AND `is_deleted` = 0
  );

INSERT INTO `sys_api` (`parent_id`, `name`, `path`, `method`, `is_permission`, `created_by`, `updated_by`, `is_deleted`)
SELECT @devops_api_id, '下载质量上传明细', '/devops/v1/quality/report/upload-entry/*/download', 'GET', 1, 'system', 'system', 0
WHERE @devops_api_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM `sys_api`
    WHERE `parent_id` = @devops_api_id AND `path` = '/devops/v1/quality/report/upload-entry/*/download' AND `method` = 'GET' AND `is_deleted` = 0
  );

INSERT INTO `sys_api` (`parent_id`, `name`, `path`, `method`, `is_permission`, `created_by`, `updated_by`, `is_deleted`)
SELECT @devops_api_id, '预览质量上传明细', '/devops/v1/quality/report/upload-entry/*/preview', 'GET', 1, 'system', 'system', 0
WHERE @devops_api_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM `sys_api`
    WHERE `parent_id` = @devops_api_id AND `path` = '/devops/v1/quality/report/upload-entry/*/preview' AND `method` = 'GET' AND `is_deleted` = 0
  );

-- 项目管理员默认拥有 DevOps 平台、菜单和 API 权限
SET @project_admin_role_id := (
  SELECT `id` FROM `sys_role`
  WHERE `code` = 'project_admin' AND `is_deleted` = 0
  LIMIT 1
);

INSERT INTO `sys_user_platform` (`user_id`, `platform_id`, `is_enable`, `status`, `is_deleted`, `create_by`, `update_by`)
SELECT DISTINCT ur.`user_id`, 3, 1, 1, 0, 'system', 'system'
FROM `sys_user_role` ur
JOIN `sys_role` r ON r.`id` = ur.`role_id`
WHERE r.`code` = 'project_admin'
  AND r.`is_deleted` = 0
  AND ur.`is_deleted` = 0
ON DUPLICATE KEY UPDATE
  `is_enable` = 1,
  `status` = 1,
  `is_deleted` = 0,
  `update_by` = 'system';

INSERT INTO `sys_role_menu` (`role_id`, `menu_id`, `is_deleted`)
SELECT @project_admin_role_id, m.`id`, 0
FROM `sys_menu` m
WHERE @project_admin_role_id IS NOT NULL
  AND m.`platform_id` = 3
  AND m.`is_deleted` = 0
  AND m.`is_enable` = 1
  AND m.`status` = 1
  AND m.`is_menu` = 1
ON DUPLICATE KEY UPDATE
  `is_deleted` = 0;

INSERT INTO `sys_role_api` (`role_id`, `api_id`, `is_deleted`)
SELECT @project_admin_role_id, a.`id`, 0
FROM `sys_api` a
WHERE @project_admin_role_id IS NOT NULL
  AND a.`path` LIKE '/devops/v1/%'
  AND a.`is_permission` = 1
  AND a.`is_deleted` = 0
ON DUPLICATE KEY UPDATE
  `is_deleted` = 0;

COMMIT;
