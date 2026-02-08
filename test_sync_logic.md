# 全量同步逻辑修复测试

## 修复内容

### 1. 添加了新的数据库查询方法

在 `onecProjectApplicationModel.go` 中添加了：
- `FindOneByWorkspaceIdNameCnResourceTypeIncludeDeleted()` - 通过中文名查询应用
- `FindByWorkspaceIdNamesResourceTypeIncludeDeleted()` - 通过中英文名智能查询应用

### 2. 修改了同步逻辑

在 `namespace_app_sync.go` 中修改了 `syncSingleResourceWithFlagger()` 方法：

#### 原逻辑问题：
```go
// 只使用 ikubeops.com/owned-by-app 注解或资源名
appName := flaggerInfo.OriginalAppName
if annotations != nil && annotations[utils.AnnotationApplicationName] != "" {
    appName = annotations[utils.AnnotationApplicationName]
}
```

#### 新逻辑：
```go
// 优先级1: ikubeops.com/owned-by-app 注解（最高优先级）
if annotations != nil && annotations[utils.AnnotationApplicationName] != "" {
    appName = annotations[utils.AnnotationApplicationName]
} else {
    // 优先级2: 使用资源标签中的应用信息
    appNameCn = annotations[utils.AnnotationApplication]    // ikubeops.com/application
    appNameEn = annotations[utils.AnnotationApplicationEn]  // ikubeops.com/application-en

    // 优先使用英文名作为应用名
    if appNameEn != "" {
        appName = appNameEn
    } else if appNameCn != "" {
        appName = appNameCn
    } else {
        // 优先级3: 使用 Flagger 识别的原始名称
        appName = flaggerInfo.OriginalAppName
    }
}
```

### 3. 添加了智能应用匹配

新增 `ensureApplicationExistsWithLabels()` 方法：
1. **智能匹配**：优先查找中英文名都匹配的应用
2. **中文名匹配**：如果没有完全匹配，尝试中文名匹配
3. **英文名匹配**：如果中文名也没匹配，尝试英文名匹配
4. **应用更新**：如果找到应用但名称不完整，自动更新中英文名
5. **软删除恢复**：如果找到的应用是软删除状态，自动恢复

### 4. 添加了正确的应用创建逻辑

新增 `createApplicationWithLabels()` 方法：
- 使用标签中的正确中英文名创建应用
- 如果缺少中文名或英文名，智能补全
- 处理并发创建的重复键错误

## 测试场景

### 场景1：资源有完整的应用标签
```yaml
annotations:
  ikubeops.com/application: 支付银行对接服务      # 中文名
  ikubeops.com/application-en: sts-test          # 英文名
```

**预期行为**：
1. 查找中文名=`支付银行对接服务` 且 英文名=`sts-test` 的应用
2. 如果找不到，分别尝试中文名或英文名匹配
3. 如果都找不到，创建新应用时使用正确的中英文名

### 场景2：资源只有中文名标签
```yaml
annotations:
  ikubeops.com/application: 支付银行对接服务      # 只有中文名
```

**预期行为**：
1. 查找中文名=`支付银行对接服务` 的应用
2. 如果找不到，创建新应用时中英文名都设置为 `支付银行对接服务`

### 场景3：资源只有英文名标签
```yaml
annotations:
  ikubeops.com/application-en: sts-test          # 只有英文名
```

**预期行为**：
1. 查找英文名=`sts-test` 的应用
2. 如果找不到，创建新应用时中sts-test`

### 场景4：资源有 owned-by-app 注解（最高优先级）
```yaml
annotations:
  ikubeops.com/owned-by-app: my-app              # 最高优先级
  ikubeops.com/application: 支付银行对接服务
  ikubeops.com/application-en: sts-test
```

**预期行为**：
1. 忽略其他标签，直接使用 `my-app` 作为应用名
2. 按原有逻辑查找和创建应用

## 修复效果

✅ **解决了原问题**：
- 现在会正确使用资源标签中的应用信息
- 支持通过中文名查找已存在的应用
- 创建新应用时使用正确的中英文名
- 自动更新不完整的应用名称信息

✅ **保持向后兼容**：
- 保留了原有的查找逻辑作为回退方案
- 保持了 `ikubeops.com/owned-by-app` 的最高优先级
- 不影响现有的 Flagger 集成功能