-- 项目管理菜单插入脚本
-- 在 sys_menu 表中插入"项目管理"菜单，使其出现在系统设置的动态菜单中
-- 此菜单依赖的组件路径为 /system/project，对应前端 src/views/system/project/index.vue

-- 获取平台ID（默认容器平台）
SET @platform_id = (SELECT id FROM sys_platform WHERE `default` = 1 OR id = 1 LIMIT 1);

-- 获取系统设置父菜单 ID（通常 parent_id = 1 是系统管理根节点）
SET @system_parent_id = (SELECT id FROM sys_menu WHERE platform_id = @platform_id AND parent_id = 0 AND `name` = 'System' LIMIT 1);

-- 插入项目管理菜单
INSERT INTO sys_menu (
    parent_id, platform_id, menu_type, `name`, `path`, component, redirect,
    title, icon, sort, is_enable, is_menu, keep_alive, is_hide,
    is_iframe, is_hide_tab, is_first_level, status, is_deleted,
    create_time, update_time, create_by, update_by,
    roles
) VALUES (
    @system_parent_id,    -- 父菜单：系统设置
    @platform_id,         -- 平台ID
    1,                    -- menu_type: 1=目录, 2=菜单
    'SystemProject',      -- name（路由名）
    'project',           -- path（路由路径）
    '/system/project',   -- component（组件路径）
    '',                   -- redirect
    '项目管理',          -- title（菜单标题）
    'ri:folder-3-line',  -- icon
    25,                   -- sort（排序，放在用户/角色之后）
    1,                    -- is_enable
    1,                    -- is_menu
    1,                    -- keep_alive
    0,                    -- is_hide
    0,                    -- is_iframe
    0,                    -- is_hide_tab
    0,                    -- is_first_level
    1,                    -- status
    0,                    -- is_deleted
    NOW(), NOW(),
    'system', 'system',
    '["R_SUPER","R_ADMIN"]'  -- roles
);

-- 如果已存在同名菜单则跳过
INSERT IGNORE INTO sys_menu (
    parent_id, platform_id, menu_type, `name`, `path`, component,
    title, icon, sort, is_enable, is_menu, keep_alive, is_hide,
    is_iframe, is_hide_tab, is_first_level, status, is_deleted,
    create_time, update_time, create_by, update_by, roles
) SELECT
    @system_parent_id, @platform_id, 1, 'SystemProject', 'project', '/system/project',
    '项目管理', 'ri:folder-3-line', 25, 1, 1, 1, 0, 0, 0, 0, 1, 0,
    NOW(), NOW(), 'system', 'system', '["R_SUPER","R_ADMIN"]'
WHERE NOT EXISTS (
    SELECT 1 FROM sys_menu 
    WHERE platform_id = @platform_id 
      AND parent_id = @system_parent_id 
      AND `name` = 'SystemProject'
);
