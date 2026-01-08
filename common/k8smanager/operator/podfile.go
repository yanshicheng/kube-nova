package operator

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"
)

// 文件操作常量
const (
	MaxFileSize        = 100 * 1024 * 1024 * 1024 // 100GB
	MaxUploadChunkSize = 10 * 1024 * 1024         // 10MB
	DefaultFileMode    = "0644"
	DefaultDirMode     = "0755"

	// 命令探测缓存时间 - 延长到30分钟
	capabilityCacheDuration = 30 * time.Minute
)

// commandCapabilities 存储容器命令能力信息
type commandCapabilities struct {
	// 基础命令可用性
	hasFind    bool
	hasLs      bool
	hasStat    bool
	hasGrep    bool
	hasFile    bool
	hasDu      bool
	hasMd5sum  bool
	hasTar     bool
	hasGzip    bool
	hasTest    bool
	hasMkdir   bool
	hasRm      bool
	hasMv      bool
	hasCp      bool
	hasCat     bool
	hasChmod   bool
	hasTail    bool
	hasDd      bool
	hasCommand bool
	hasWhich   bool

	// find 命令能力
	findSupportsPrintf bool
	findSupportsType   bool

	// ls 命令能力
	lsSupportsTimeStyle bool
	lsSupportsFullTime  bool
	lsSupportsLongList  bool

	// stat 命令能力
	statSupportsFormat bool
	statSupportsC      bool

	// grep 命令能力
	grepSupportsContext bool

	// 环境类型检测
	isBusyBox bool
	isGNU     bool
	shellType string

	// 检测时间戳和容器标识
	detectedAt   time.Time
	containerKey string
}

// capabilityCache 全局命令能力缓存
var (
	capabilityCache      = make(map[string]*commandCapabilities)
	capabilityCacheMutex sync.RWMutex
)

// getContainerKey 生成容器唯一键
func getContainerKey(namespace, podName, container string) string {
	return fmt.Sprintf("%s/%s/%s", namespace, podName, container)
}

// detectCommandCapabilities 探测容器命令能力
func (p *podOperator) detectCommandCapabilities(ctx context.Context, namespace, podName, container string) (*commandCapabilities, error) {
	logger := logx.WithContext(ctx)
	containerKey := getContainerKey(namespace, podName, container)

	// 检查缓存
	capabilityCacheMutex.RLock()
	if cached, ok := capabilityCache[containerKey]; ok {
		if time.Since(cached.detectedAt) < capabilityCacheDuration {
			capabilityCacheMutex.RUnlock()
			logger.Debugf("使用缓存的命令能力信息: %s", containerKey)
			return cached, nil
		}
	}
	capabilityCacheMutex.RUnlock()

	logger.Infof("开始探测容器命令能力: %s", containerKey)

	caps := &commandCapabilities{
		detectedAt:   time.Now(),
		containerKey: containerKey,
	}

	// 使用单个脚本批量检测所有命令和能力
	detectScript := `
cmd_exists() {
    command -v "$1" >/dev/null 2>&1 && echo "1" || echo "0"
}
echo "has_command=$(cmd_exists command)"
echo "has_which=$(cmd_exists which)"
echo "has_find=$(cmd_exists find)"
echo "has_ls=$(cmd_exists ls)"
echo "has_stat=$(cmd_exists stat)"
echo "has_grep=$(cmd_exists grep)"
echo "has_file=$(cmd_exists file)"
echo "has_du=$(cmd_exists du)"
echo "has_md5sum=$(cmd_exists md5sum)"
echo "has_tar=$(cmd_exists tar)"
echo "has_gzip=$(cmd_exists gzip)"
echo "has_test=$(cmd_exists test)"
echo "has_mkdir=$(cmd_exists mkdir)"
echo "has_rm=$(cmd_exists rm)"
echo "has_mv=$(cmd_exists mv)"
echo "has_cp=$(cmd_exists cp)"
echo "has_cat=$(cmd_exists cat)"
echo "has_chmod=$(cmd_exists chmod)"
echo "has_tail=$(cmd_exists tail)"
echo "has_dd=$(cmd_exists dd)"
if ls --version 2>&1 | head -1 | grep -q "BusyBox"; then
    echo "is_busybox=1"
else
    echo "is_busybox=0"
fi
if command -v find >/dev/null 2>&1; then
    if find / -maxdepth 0 -printf '%p' 2>/dev/null | grep -q '/'; then
        echo "find_printf=1"
    else
        echo "find_printf=0"
    fi
    if find / -maxdepth 0 -type d >/dev/null 2>&1; then
        echo "find_type=1"
    else
        echo "find_type=0"
    fi
else
    echo "find_printf=0"
    echo "find_type=0"
fi
if command -v ls >/dev/null 2>&1; then
    if ls --time-style=+%s / >/dev/null 2>&1; then
        echo "ls_timestyle=1"
    else
        echo "ls_timestyle=0"
    fi
    if ls --full-time / >/dev/null 2>&1; then
        echo "ls_fulltime=1"
    else
        echo "ls_fulltime=0"
    fi
    if ls -la / >/dev/null 2>&1; then
        echo "ls_longlist=1"
    else
        echo "ls_longlist=0"
    fi
else
    echo "ls_timestyle=0"
    echo "ls_fulltime=0"
    echo "ls_longlist=0"
fi
if command -v stat >/dev/null 2>&1; then
    if stat --format=%n / >/dev/null 2>&1; then
        echo "stat_format=1"
    else
        echo "stat_format=0"
    fi
    if stat -c %n / >/dev/null 2>&1; then
        echo "stat_c=1"
    else
        echo "stat_c=0"
    fi
else
    echo "stat_format=0"
    echo "stat_c=0"
fi
if command -v grep >/dev/null 2>&1; then
    echo "test" > /tmp/.cap_test_$$ 2>/dev/null
    if grep -B 1 -A 1 test /tmp/.cap_test_$$ >/dev/null 2>&1; then
        echo "grep_context=1"
    else
        echo "grep_context=0"
    fi
    rm -f /tmp/.cap_test_$$ 2>/dev/null
else
    echo "grep_context=0"
fi
for sh in bash ash dash sh; do
    if command -v $sh >/dev/null 2>&1; then
        echo "shell_type=$sh"
        break
    fi
done
`

	cmd := []string{"sh", "-c", detectScript}
	stdout, _, err := p.ExecCommand(namespace, podName, container, cmd)
	if err != nil {
		logger.Errorf("批量命令检测失败: %v，使用最小化检测", err)
		// 降级到最小化检测
		return p.detectMinimalCapabilities(namespace, podName, container)
	}

	// 解析检测结果
	p.parseCapabilityOutput(stdout, caps)

	// 设置 isGNU
	caps.isGNU = !caps.isBusyBox

	// 缓存结果
	capabilityCacheMutex.Lock()
	capabilityCache[containerKey] = caps
	capabilityCacheMutex.Unlock()

	logger.Infof("命令能力探测完成: BusyBox=%v, find-printf=%v, ls-timestyle=%v",
		caps.isBusyBox, caps.findSupportsPrintf, caps.lsSupportsTimeStyle)

	return caps, nil
}

// parseCapabilityOutput 解析能力检测输出
func (p *podOperator) parseCapabilityOutput(output string, caps *commandCapabilities) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1]) == "1"

		switch key {
		case "has_command":
			caps.hasCommand = value
		case "has_which":
			caps.hasWhich = value
		case "has_find":
			caps.hasFind = value
		case "has_ls":
			caps.hasLs = value
		case "has_stat":
			caps.hasStat = value
		case "has_grep":
			caps.hasGrep = value
		case "has_file":
			caps.hasFile = value
		case "has_du":
			caps.hasDu = value
		case "has_md5sum":
			caps.hasMd5sum = value
		case "has_tar":
			caps.hasTar = value
		case "has_gzip":
			caps.hasGzip = value
		case "has_test":
			caps.hasTest = value
		case "has_mkdir":
			caps.hasMkdir = value
		case "has_rm":
			caps.hasRm = value
		case "has_mv":
			caps.hasMv = value
		case "has_cp":
			caps.hasCp = value
		case "has_cat":
			caps.hasCat = value
		case "has_chmod":
			caps.hasChmod = value
		case "has_tail":
			caps.hasTail = value
		case "has_dd":
			caps.hasDd = value
		case "is_busybox":
			caps.isBusyBox = value
		case "find_printf":
			caps.findSupportsPrintf = value
		case "find_type":
			caps.findSupportsType = value
		case "ls_timestyle":
			caps.lsSupportsTimeStyle = value
		case "ls_fulltime":
			caps.lsSupportsFullTime = value
		case "ls_longlist":
			caps.lsSupportsLongList = value
		case "stat_format":
			caps.statSupportsFormat = value
		case "stat_c":
			caps.statSupportsC = value
		case "grep_context":
			caps.grepSupportsContext = value
		case "shell_type":
			caps.shellType = strings.TrimSpace(parts[1])
		}
	}

	// 默认值
	if caps.shellType == "" {
		caps.shellType = "sh"
	}
}

// detectMinimalCapabilities 最小化能力检测（降级方案）
func (p *podOperator) detectMinimalCapabilities(namespace, podName, container string) (*commandCapabilities, error) {
	caps := &commandCapabilities{
		detectedAt:   time.Now(),
		containerKey: getContainerKey(namespace, podName, container),
		shellType:    "sh",
	}

	// 只检测最关键的命令：ls
	cmd := []string{"ls", "-la", "/"}
	_, _, err := p.ExecCommand(namespace, podName, container, cmd)
	if err == nil {
		caps.hasLs = true
		caps.lsSupportsLongList = true
	}

	return caps, nil
}

// ========== 错误处理辅助函数 ==========

// parsePathError 解析路径相关错误，返回友好的错误信息
func (p *podOperator) parsePathError(stderr, path string) error {
	stderrLower := strings.ToLower(stderr)

	if strings.Contains(stderrLower, "no such file") ||
		strings.Contains(stderrLower, "not found") ||
		strings.Contains(stderrLower, "cannot access") ||
		strings.Contains(stderrLower, "does not exist") {
		return fmt.Errorf("路径不存在: %s", path)
	}

	if strings.Contains(stderrLower, "permission denied") ||
		strings.Contains(stderrLower, "operation not permitted") {
		return fmt.Errorf("没有权限访问: %s", path)
	}

	if strings.Contains(stderrLower, "not a directory") {
		return fmt.Errorf("不是目录: %s", path)
	}

	if strings.Contains(stderrLower, "is a directory") {
		return fmt.Errorf("是目录而非文件: %s", path)
	}

	return nil
}

// ========== 智能文件列表（多层降级）==========

// ListFiles 列出Pod中的文件和目录（智能降级版本）
func (p *podOperator) ListFiles(ctx context.Context, namespace, podName, container, dirPath string, opts *types.FileListOptions) (*types.FileListResult, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("开始列出Pod文件: namespace=%s, pod=%s, container=%s, path=%s",
		namespace, podName, container, dirPath)

	if namespace == "" || podName == "" {
		logger.Error("列出文件失败：命名空间和Pod名称不能为空")
		return nil, fmt.Errorf("命名空间和Pod名称不能为空")
	}

	// 确保有有效的容器名称
	var err error
	container, err = p.ensureContainer(namespace, podName, container)
	if err != nil {
		return nil, err
	}

	if dirPath == "" {
		dirPath = "/"
	}

	if opts == nil {
		opts = &types.FileListOptions{
			ShowHidden: false,
			SortBy:     types.SortByName,
			SortDesc:   false,
		}
	}

	// 探测命令能力（使用缓存，几乎无开销）
	caps, err := p.detectCommandCapabilities(ctx, namespace, podName, container)
	if err != nil {
		logger.Errorf("探测命令能力失败，使用默认方法: %v", err)
		caps = &commandCapabilities{hasLs: true, lsSupportsLongList: true}
	}

	// 智能选择列表方法（多层降级）
	var result *types.FileListResult
	var lastErr error

	// 策略1: GNU find with -printf (最优)
	if caps.hasFind && caps.findSupportsPrintf {
		logger.Debug("策略1: 使用 GNU find -printf")
		result, err = p.listFilesWithGNUFind(ctx, namespace, podName, container, dirPath, opts)
		if err == nil {
			return result, nil
		}
		lastErr = err
		// 如果是路径错误，直接返回
		if pathErr := p.parsePathError(err.Error(), dirPath); pathErr != nil {
			return nil, pathErr
		}
		logger.Debugf("GNU find 失败: %v, 尝试降级", err)
	}

	// 策略2: BusyBox find
	if caps.hasFind && !caps.findSupportsPrintf {
		logger.Debug("策略2: 使用 BusyBox find")
		result, err = p.listFilesWithBusyBoxFind(ctx, namespace, podName, container, dirPath, opts)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if pathErr := p.parsePathError(err.Error(), dirPath); pathErr != nil {
			return nil, pathErr
		}
		logger.Debugf("BusyBox find 失败: %v, 尝试降级", err)
	}

	// 策略3: GNU ls with --time-style
	if caps.hasLs && caps.lsSupportsTimeStyle {
		logger.Debug("策略3: 使用 GNU ls --time-style")
		result, err = p.listFilesWithGNULs(ctx, namespace, podName, container, dirPath, opts)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if pathErr := p.parsePathError(err.Error(), dirPath); pathErr != nil {
			return nil, pathErr
		}
		logger.Debugf("GNU ls 失败: %v, 尝试降级", err)
	}

	// 策略4: ls --full-time
	if caps.hasLs && caps.lsSupportsFullTime && !caps.lsSupportsTimeStyle {
		logger.Debug("策略4: 使用 ls --full-time")
		result, err = p.listFilesWithFullTimeLs(ctx, namespace, podName, container, dirPath, opts)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if pathErr := p.parsePathError(err.Error(), dirPath); pathErr != nil {
			return nil, pathErr
		}
		logger.Debugf("ls --full-time 失败: %v, 尝试降级", err)
	}

	// 策略5: BusyBox ls (最后的降级)
	if caps.hasLs {
		logger.Debug("策略5: 使用基础 ls -la")
		result, err = p.listFilesWithBusyBoxLs(ctx, namespace, podName, container, dirPath, opts)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if pathErr := p.parsePathError(err.Error(), dirPath); pathErr != nil {
			return nil, pathErr
		}
		logger.Debugf("BusyBox ls 失败: %v", err)
	}

	// 所有方法都失败
	logger.Error("所有文件列表策略都失败")

	if lastErr != nil {
		return nil, fmt.Errorf("无法列出文件: %v", lastErr)
	}

	return nil, fmt.Errorf("无法列出文件: 容器中缺少必要的命令 (find/ls)")
}

// listFilesWithGNUFind 使用 GNU find -printf 列出文件
func (p *podOperator) listFilesWithGNUFind(ctx context.Context, namespace, podName, container, dirPath string, opts *types.FileListOptions) (*types.FileListResult, error) {
	maxdepth := "1"
	if opts.Recursive && opts.MaxDepth > 0 {
		maxdepth = strconv.Itoa(opts.MaxDepth)
	}

	// 使用 2>/dev/null 忽略权限错误等，但保留主要输出
	findCmd := []string{
		"sh", "-c",
		fmt.Sprintf("find '%s' -maxdepth %s -printf '%%p|%%s|%%T@|%%u|%%g|%%m|%%y\\n' 2>/dev/null", dirPath, maxdepth),
	}

	stdout, stderr, err := p.ExecCommand(namespace, podName, container, findCmd)
	// find 命令即使有部分错误也可能返回部分结果
	// 只有当 stdout 为空且有 stderr 时才认为是真正的错误
	if stdout == "" && err != nil {
		if pathErr := p.parsePathError(stderr, dirPath); pathErr != nil {
			return nil, pathErr
		}
		return nil, fmt.Errorf("find 命令失败: %v", err)
	}

	files, err := p.parseFindOutput(stdout, dirPath, opts)
	if err != nil {
		return nil, err
	}

	return p.buildFileListResult(files, dirPath, container), nil
}

// listFilesWithBusyBoxFind 使用 BusyBox find 列出文件
func (p *podOperator) listFilesWithBusyBoxFind(ctx context.Context, namespace, podName, container, dirPath string, opts *types.FileListOptions) (*types.FileListResult, error) {
	maxdepth := "1"
	if opts.Recursive && opts.MaxDepth > 0 {
		maxdepth = strconv.Itoa(opts.MaxDepth)
	}

	// BusyBox find 不支持 -printf，使用 -exec ls -ld
	findCmd := []string{
		"sh", "-c",
		fmt.Sprintf("find '%s' -maxdepth %s -exec ls -ld {} \\; 2>/dev/null", dirPath, maxdepth),
	}

	stdout, stderr, err := p.ExecCommand(namespace, podName, container, findCmd)
	if stdout == "" && err != nil {
		if pathErr := p.parsePathError(stderr, dirPath); pathErr != nil {
			return nil, pathErr
		}
		return nil, fmt.Errorf("find 命令失败: %v", err)
	}

	files, err := p.parseListOutput(stdout, dirPath)
	if err != nil {
		return nil, err
	}

	files = p.filterAndSortFiles(files, opts)
	return p.buildFileListResult(files, dirPath, container), nil
}

// listFilesWithGNULs 使用 GNU ls --time-style 列出文件
func (p *podOperator) listFilesWithGNULs(ctx context.Context, namespace, podName, container, dirPath string, opts *types.FileListOptions) (*types.FileListResult, error) {
	lsCmd := []string{"ls", "-la", "--time-style=+%s", dirPath}
	stdout, stderr, err := p.ExecCommand(namespace, podName, container, lsCmd)
	if err != nil {
		if pathErr := p.parsePathError(stderr, dirPath); pathErr != nil {
			return nil, pathErr
		}
		return nil, fmt.Errorf("ls 命令失败: %v", err)
	}

	files, err := p.parseLsWithTimestamp(stdout, dirPath)
	if err != nil {
		return nil, err
	}

	files = p.filterAndSortFiles(files, opts)
	return p.buildFileListResult(files, dirPath, container), nil
}

// listFilesWithFullTimeLs 使用 ls --full-time 列出文件
func (p *podOperator) listFilesWithFullTimeLs(ctx context.Context, namespace, podName, container, dirPath string, opts *types.FileListOptions) (*types.FileListResult, error) {
	lsCmd := []string{"ls", "-la", "--full-time", dirPath}
	stdout, stderr, err := p.ExecCommand(namespace, podName, container, lsCmd)
	if err != nil {
		if pathErr := p.parsePathError(stderr, dirPath); pathErr != nil {
			return nil, pathErr
		}
		return nil, fmt.Errorf("ls 命令失败: %v", err)
	}

	files, err := p.parseLsFullTime(stdout, dirPath)
	if err != nil {
		return nil, err
	}

	files = p.filterAndSortFiles(files, opts)
	return p.buildFileListResult(files, dirPath, container), nil
}

// listFilesWithBusyBoxLs 使用 BusyBox ls 列出文件
func (p *podOperator) listFilesWithBusyBoxLs(ctx context.Context, namespace, podName, container, dirPath string, opts *types.FileListOptions) (*types.FileListResult, error) {
	lsCmd := []string{"ls", "-la", dirPath}
	stdout, stderr, err := p.ExecCommand(namespace, podName, container, lsCmd)
	if err != nil {
		if pathErr := p.parsePathError(stderr, dirPath); pathErr != nil {
			return nil, pathErr
		}
		return nil, fmt.Errorf("ls 命令失败: %v", err)
	}

	files, err := p.parseBusyBoxLs(stdout, dirPath)
	if err != nil {
		return nil, err
	}

	files = p.filterAndSortFiles(files, opts)
	return p.buildFileListResult(files, dirPath, container), nil
}

// ========== 解析方法 ==========

// parseFindOutput 解析 GNU find -printf 输出
// find -printf '%p|%s|%T@|%u|%g|%m|%y\n' 输出完整路径
func (p *podOperator) parseFindOutput(output, basePath string, opts *types.FileListOptions) ([]types.FileInfo, error) {
	files := []types.FileInfo{}
	lines := strings.Split(output, "\n")

	// 规范化 basePath，确保路径比较正确
	basePath = filepath.Clean(basePath)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, "|")
		if len(fields) < 7 {
			continue
		}

		// 规范化路径
		fullPath := filepath.Clean(fields[0])

		if fullPath == basePath {
			continue
		}

		// 获取真正的文件名
		name := filepath.Base(fullPath)
		if name == "." || name == ".." {
			continue
		}

		// 隐藏文件过滤
		if !opts.ShowHidden && strings.HasPrefix(name, ".") {
			continue
		}

		// 搜索过滤
		if opts.Search != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(opts.Search)) {
			continue
		}

		size, _ := strconv.ParseInt(fields[1], 10, 64)
		timestamp, _ := strconv.ParseFloat(fields[2], 64)
		mode, _ := strconv.ParseInt(fields[5], 8, 32)

		fileType := fields[6]
		isDir := fileType == "d"
		isLink := fileType == "l"

		fileInfo := types.FileInfo{
			Name:   name,     // 使用 filepath.Base 获取的文件名
			Path:   fullPath, // 使用规范化后的完整路径
			Size:   size,
			Mode:   fmt.Sprintf("%o", mode),
			IsDir:  isDir,
			IsLink: isLink,
			Owner:  fields[3],
			Group:  fields[4],
		}

		if timestamp > 0 {
			fileInfo.ModTime = time.Unix(int64(timestamp), 0)
		}

		fileInfo.Permissions = p.parsePermissions(mode)
		fileInfo.IsReadable = (mode & 0400) != 0
		fileInfo.IsWritable = (mode & 0200) != 0
		fileInfo.IsExecutable = (mode & 0100) != 0
		fileInfo.MimeType = p.guessMimeType(name, isDir)

		files = append(files, fileInfo)
	}

	// 文件类型过滤
	if len(opts.FileTypes) > 0 {
		files = p.filterByFileTypes(files, opts.FileTypes)
	}

	// 排序
	p.sortFiles(files, opts)

	// 分页
	if opts.Limit > 0 {
		files = p.applyPagination(files, opts.Offset, opts.Limit)
	}

	return files, nil
}

// parseLsWithTimestamp 解析 ls -la --time-style=+%s 的输出
// 格式: permissions links owner group size timestamp name...
func (p *podOperator) parseLsWithTimestamp(output string, basePath string) ([]types.FileInfo, error) {
	files := []types.FileInfo{}
	lines := strings.Split(output, "\n")

	// 规范化 basePath
	basePath = filepath.Clean(basePath)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}

		fields := strings.Fields(line)
		// ls -la --time-style=+%s 输出: perm links owner group size timestamp name
		// 例如: drwxr-xr-x 2 root root 4096 1732924440 tmp
		if len(fields) < 7 {
			continue
		}

		// 文件名从第7个字段开始 (索引6)
		name := strings.Join(fields[6:], " ")

		// 处理符号链接
		linkTarget := ""
		if strings.Contains(name, " -> ") {
			parts := strings.SplitN(name, " -> ", 2)
			name = parts[0]
			if len(parts) > 1 {
				linkTarget = parts[1]
			}
		}

		// 跳过 . 和 ..
		if name == "." || name == ".." {
			continue
		}

		// 构建完整路径
		fullPath := filepath.Join(basePath, name)

		fileInfo := types.FileInfo{
			Name:       name,
			Path:       fullPath,
			Mode:       fields[0],
			IsDir:      strings.HasPrefix(fields[0], "d"),
			IsLink:     strings.HasPrefix(fields[0], "l"),
			LinkTarget: linkTarget,
			Owner:      fields[2],
			Group:      fields[3],
		}

		size, _ := strconv.ParseInt(fields[4], 10, 64)
		fileInfo.Size = size

		// 时间戳格式 (第6个字段，索引5)
		if timestamp, err := strconv.ParseInt(fields[5], 10, 64); err == nil {
			fileInfo.ModTime = time.Unix(timestamp, 0)
		} else {
			fileInfo.ModTime = time.Now()
		}

		fileInfo.Permissions = p.parsePermissionsFromString(fields[0])
		fileInfo.IsReadable = len(fields[0]) > 1 && fields[0][1] == 'r'
		fileInfo.IsWritable = len(fields[0]) > 2 && fields[0][2] == 'w'
		fileInfo.IsExecutable = len(fields[0]) > 3 && fields[0][3] == 'x'
		fileInfo.MimeType = p.guessMimeType(name, fileInfo.IsDir)

		files = append(files, fileInfo)
	}

	return files, nil
}

// parseLsFullTime 解析 ls -la --full-time 的输出
// 格式: permissions links owner group size date time timezone name...
// 例如: drwxr-xr-x 2 root root 4096 2024-01-15 10:30:45.123456789 +0800 tmp
func (p *podOperator) parseLsFullTime(output string, basePath string) ([]types.FileInfo, error) {
	files := []types.FileInfo{}
	lines := strings.Split(output, "\n")

	// 规范化 basePath
	basePath = filepath.Clean(basePath)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		// --full-time 格式: permissions links owner group size date time timezone name...
		// 文件名从第9个字段开始 (索引8)
		name := strings.Join(fields[8:], " ")

		// 处理符号链接
		linkTarget := ""
		if strings.Contains(name, " -> ") {
			parts := strings.SplitN(name, " -> ", 2)
			name = parts[0]
			if len(parts) > 1 {
				linkTarget = parts[1]
			}
		}

		// 跳过 . 和 ..
		if name == "." || name == ".." {
			continue
		}

		// 构建完整路径
		fullPath := filepath.Join(basePath, name)

		fileInfo := types.FileInfo{
			Name:       name,
			Path:       fullPath,
			Mode:       fields[0],
			IsDir:      strings.HasPrefix(fields[0], "d"),
			IsLink:     strings.HasPrefix(fields[0], "l"),
			LinkTarget: linkTarget,
			Owner:      fields[2],
			Group:      fields[3],
		}

		size, _ := strconv.ParseInt(fields[4], 10, 64)
		fileInfo.Size = size

		// 解析完整时间: 2024-01-15 10:30:45.123456789 +0800
		timeStr := fmt.Sprintf("%s %s", fields[5], fields[6])
		if modTime, err := time.Parse("2006-01-02 15:04:05.999999999", timeStr); err == nil {
			fileInfo.ModTime = modTime
		} else if modTime, err := time.Parse("2006-01-02 15:04:05", timeStr); err == nil {
			fileInfo.ModTime = modTime
		} else {
			fileInfo.ModTime = time.Now()
		}

		fileInfo.Permissions = p.parsePermissionsFromString(fields[0])
		fileInfo.IsReadable = len(fields[0]) > 1 && fields[0][1] == 'r'
		fileInfo.IsWritable = len(fields[0]) > 2 && fields[0][2] == 'w'
		fileInfo.IsExecutable = len(fields[0]) > 3 && fields[0][3] == 'x'
		fileInfo.MimeType = p.guessMimeType(name, fileInfo.IsDir)

		files = append(files, fileInfo)
	}

	return files, nil
}

// parseBusyBoxLs 解析 ls -la 的输出
// 注意：ls -la /tmp 输出的是相对文件名，不是完整路径
// 例如: drwxrwxrwt 2 root root 4096 Nov 30 06:14 .
func (p *podOperator) parseBusyBoxLs(output string, basePath string) ([]types.FileInfo, error) {
	files := []types.FileInfo{}
	lines := strings.Split(output, "\n")

	// 规范化 basePath
	basePath = filepath.Clean(basePath)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		// BusyBox ls -la 格式: permissions links owner group size month day time/year name
		// 注意：这里 name 是相对文件名
		name := strings.Join(fields[8:], " ")

		// 处理符号链接: name -> target
		linkTarget := ""
		if strings.Contains(name, " -> ") {
			parts := strings.SplitN(name, " -> ", 2)
			name = parts[0]
			if len(parts) > 1 {
				linkTarget = parts[1]
			}
		}

		// 跳过 . 和 ..
		if name == "." || name == ".." {
			continue
		}

		// 构建完整路径
		fullPath := filepath.Join(basePath, name)

		fileInfo := types.FileInfo{
			Name:       name,
			Path:       fullPath,
			Mode:       fields[0],
			IsDir:      strings.HasPrefix(fields[0], "d"),
			IsLink:     strings.HasPrefix(fields[0], "l"),
			LinkTarget: linkTarget,
			Owner:      fields[2],
			Group:      fields[3],
		}

		size, _ := strconv.ParseInt(fields[4], 10, 64)
		fileInfo.Size = size

		// 解析时间: "Jan 15 10:30" 或 "Jan 15  2024"
		timeStr := fmt.Sprintf("%s %s %s", fields[5], fields[6], fields[7])
		fileInfo.ModTime = p.parseBusyBoxTime(timeStr)

		fileInfo.Permissions = p.parsePermissionsFromString(fields[0])
		fileInfo.IsReadable = len(fields[0]) > 1 && fields[0][1] == 'r'
		fileInfo.IsWritable = len(fields[0]) > 2 && fields[0][2] == 'w'
		fileInfo.IsExecutable = len(fields[0]) > 3 && fields[0][3] == 'x'
		fileInfo.MimeType = p.guessMimeType(name, fileInfo.IsDir)

		files = append(files, fileInfo)
	}

	return files, nil
}

// parseListOutput 解析 find -exec ls -ld 的输出
// 注意：find -exec ls -ld {} \; 输出的是完整路径，不是相对路径！
// 例如: drwxrwxrwt 2 root root 4096 Nov 30 06:14 /tmp
func (p *podOperator) parseListOutput(output string, basePath string) ([]types.FileInfo, error) {
	files := []types.FileInfo{}
	lines := strings.Split(output, "\n")

	// 规范化 basePath，确保路径比较正确
	basePath = filepath.Clean(basePath)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		// ls -ld 输出格式: permissions links owner group size month day time/year path
		// 注意：find -exec ls -ld {} \; 输出的是完整路径！
		fullPath := strings.Join(fields[8:], " ")

		// 处理符号链接: /path/to/link -> /path/to/target
		linkTarget := ""
		if strings.Contains(fullPath, " -> ") {
			parts := strings.SplitN(fullPath, " -> ", 2)
			fullPath = parts[0]
			if len(parts) > 1 {
				linkTarget = parts[1]
			}
		}

		// 规范化路径
		fullPath = filepath.Clean(fullPath)

		if fullPath == basePath {
			continue
		}

		// 获取真正的文件名
		name := filepath.Base(fullPath)
		if name == "." || name == ".." {
			continue
		}

		fileInfo := types.FileInfo{
			Name:       name,     // 使用 filepath.Base 获取的文件名
			Path:       fullPath, // 直接使用 ls -ld 输出的完整路径
			Mode:       fields[0],
			IsDir:      strings.HasPrefix(fields[0], "d"),
			IsLink:     strings.HasPrefix(fields[0], "l"),
			LinkTarget: linkTarget,
			Owner:      fields[2],
			Group:      fields[3],
		}

		size, _ := strconv.ParseInt(fields[4], 10, 64)
		fileInfo.Size = size

		// 解析时间
		timeStr := fmt.Sprintf("%s %s %s", fields[5], fields[6], fields[7])
		fileInfo.ModTime = p.parseBusyBoxTime(timeStr)

		fileInfo.Permissions = p.parsePermissionsFromString(fields[0])
		fileInfo.IsReadable = len(fields[0]) > 1 && fields[0][1] == 'r'
		fileInfo.IsWritable = len(fields[0]) > 2 && fields[0][2] == 'w'
		fileInfo.IsExecutable = len(fields[0]) > 3 && fields[0][3] == 'x'
		fileInfo.MimeType = p.guessMimeType(name, fileInfo.IsDir)

		files = append(files, fileInfo)
	}

	return files, nil
}

// parseBusyBoxTime 解析 BusyBox 时间格式
func (p *podOperator) parseBusyBoxTime(timeStr string) time.Time {
	timeStr = strings.TrimSpace(timeStr)

	formats := []string{
		"Jan 2 15:04",
		"Jan 2 2006",
		"Jan _2 15:04",
		"Jan _2 2006",
		"2006-01-02 15:04",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			// 如果格式中没有年份，补充当前年份
			if !strings.Contains(format, "2006") {
				now := time.Now()
				t = time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
				// 如果解析出的时间在未来，说明应该是去年
				if t.After(now) {
					t = t.AddDate(-1, 0, 0)
				}
			}
			return t
		}
	}

	return time.Now()
}

// parsePermissions 从八进制模式解析权限
func (p *podOperator) parsePermissions(mode int64) types.FilePermission {
	return types.FilePermission{
		User: types.PermissionBits{
			Read:    (mode & 0400) != 0,
			Write:   (mode & 0200) != 0,
			Execute: (mode & 0100) != 0,
		},
		Group: types.PermissionBits{
			Read:    (mode & 0040) != 0,
			Write:   (mode & 0020) != 0,
			Execute: (mode & 0010) != 0,
		},
		Other: types.PermissionBits{
			Read:    (mode & 0004) != 0,
			Write:   (mode & 0002) != 0,
			Execute: (mode & 0001) != 0,
		},
	}
}

// parsePermissionsFromString 从权限字符串解析权限
func (p *podOperator) parsePermissionsFromString(permStr string) types.FilePermission {
	if len(permStr) < 10 {
		return types.FilePermission{}
	}

	return types.FilePermission{
		User: types.PermissionBits{
			Read:    permStr[1] == 'r',
			Write:   permStr[2] == 'w',
			Execute: permStr[3] == 'x' || permStr[3] == 's' || permStr[3] == 'S',
		},
		Group: types.PermissionBits{
			Read:    permStr[4] == 'r',
			Write:   permStr[5] == 'w',
			Execute: permStr[6] == 'x' || permStr[6] == 's' || permStr[6] == 'S',
		},
		Other: types.PermissionBits{
			Read:    permStr[7] == 'r',
			Write:   permStr[8] == 'w',
			Execute: permStr[9] == 'x' || permStr[9] == 't' || permStr[9] == 'T',
		},
	}
}

// ========== 智能 GetFileInfo（多层降级）==========

// GetFileInfo 获取单个文件或目录的详细信息（智能降级）
func (p *podOperator) GetFileInfo(ctx context.Context, namespace, podName, container, filePath string) (*types.FileInfo, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("获取文件信息: namespace=%s, pod=%s, container=%s, file=%s",
		namespace, podName, container, filePath)

	var err error
	container, err = p.ensureContainer(namespace, podName, container)
	if err != nil {
		return nil, err
	}

	// 探测命令能力
	caps, err := p.detectCommandCapabilities(ctx, namespace, podName, container)
	if err != nil {
		logger.Errorf("探测命令能力失败，使用ls: %v", err)
		return p.getFileInfoWithLs(namespace, podName, container, filePath)
	}

	var fileInfo *types.FileInfo

	// 策略1: GNU stat --format
	if caps.hasStat && caps.statSupportsFormat {
		fileInfo, err = p.getFileInfoWithGNUStat(namespace, podName, container, filePath)
		if err == nil {
			return p.enrichFileInfo(ctx, namespace, podName, container, fileInfo, caps)
		}
		if pathErr := p.parsePathError(err.Error(), filePath); pathErr != nil {
			return nil, pathErr
		}
		logger.Debugf("GNU stat 失败: %v, 尝试降级", err)
	}

	// 策略2: BusyBox stat -c
	if caps.hasStat && caps.statSupportsC {
		fileInfo, err = p.getFileInfoWithBusyBoxStat(namespace, podName, container, filePath)
		if err == nil {
			return p.enrichFileInfo(ctx, namespace, podName, container, fileInfo, caps)
		}
		if pathErr := p.parsePathError(err.Error(), filePath); pathErr != nil {
			return nil, pathErr
		}
		logger.Debugf("BusyBox stat 失败: %v, 尝试降级", err)
	}

	// 策略3: 使用 ls -ld 解析
	fileInfo, err = p.getFileInfoWithLs(namespace, podName, container, filePath)
	if err == nil {
		return p.enrichFileInfo(ctx, namespace, podName, container, fileInfo, caps)
	}

	if pathErr := p.parsePathError(err.Error(), filePath); pathErr != nil {
		return nil, pathErr
	}

	return nil, fmt.Errorf("无法获取文件信息: %v", err)
}

// getFileInfoWithGNUStat 使用 GNU stat 获取文件信息
func (p *podOperator) getFileInfoWithGNUStat(namespace, podName, container, filePath string) (*types.FileInfo, error) {
	statCmd := []string{"stat", "--format", "%n|%F|%s|%Y|%U|%G|%a|%h", filePath}
	stdout, stderr, err := p.ExecCommand(namespace, podName, container, statCmd)
	if err != nil {
		return nil, fmt.Errorf("stat 失败: %v, stderr: %s", err, stderr)
	}
	return p.parseStatOutput(stdout, filePath)
}

// getFileInfoWithBusyBoxStat 使用 BusyBox stat 获取文件信息
func (p *podOperator) getFileInfoWithBusyBoxStat(namespace, podName, container, filePath string) (*types.FileInfo, error) {
	statCmd := []string{"stat", "-c", "%n|%F|%s|%Y|%U|%G|%a|%h", filePath}
	stdout, stderr, err := p.ExecCommand(namespace, podName, container, statCmd)
	if err != nil {
		return nil, fmt.Errorf("stat 失败: %v, stderr: %s", err, stderr)
	}
	return p.parseStatOutput(stdout, filePath)
}

// getFileInfoWithLs 使用 ls -ld 获取文件信息
func (p *podOperator) getFileInfoWithLs(namespace, podName, container, filePath string) (*types.FileInfo, error) {
	lsCmd := []string{"ls", "-ld", filePath}
	stdout, stderr, err := p.ExecCommand(namespace, podName, container, lsCmd)
	if err != nil {
		return nil, fmt.Errorf("ls 失败: %v, stderr: %s", err, stderr)
	}

	stdout = strings.TrimSpace(stdout)
	fields := strings.Fields(stdout)
	if len(fields) < 8 {
		return nil, fmt.Errorf("ls 输出格式异常: %s", stdout)
	}

	name := filepath.Base(filePath)
	fileInfo := &types.FileInfo{
		Name:   name,
		Path:   filePath,
		Mode:   fields[0],
		IsDir:  strings.HasPrefix(fields[0], "d"),
		IsLink: strings.HasPrefix(fields[0], "l"),
		Owner:  fields[2],
		Group:  fields[3],
	}

	size, _ := strconv.ParseInt(fields[4], 10, 64)
	fileInfo.Size = size

	fileInfo.Permissions = p.parsePermissionsFromString(fields[0])
	fileInfo.IsReadable = len(fields[0]) > 1 && fields[0][1] == 'r'
	fileInfo.IsWritable = len(fields[0]) > 2 && fields[0][2] == 'w'
	fileInfo.IsExecutable = len(fields[0]) > 3 && fields[0][3] == 'x'

	timeStr := fmt.Sprintf("%s %s %s", fields[5], fields[6], fields[7])
	fileInfo.ModTime = p.parseBusyBoxTime(timeStr)
	fileInfo.MimeType = p.guessMimeType(name, fileInfo.IsDir)

	return fileInfo, nil
}

// enrichFileInfo 丰富文件信息（添加 MIME 类型等）
func (p *podOperator) enrichFileInfo(ctx context.Context, namespace, podName, container string, fileInfo *types.FileInfo, caps *commandCapabilities) (*types.FileInfo, error) {
	// 如果已经有 MIME 类型，直接返回
	if fileInfo.MimeType != "" {
		return fileInfo, nil
	}

	if !fileInfo.IsDir && caps != nil && caps.hasFile {
		mimeCmd := []string{"file", "-b", "--mime-type", fileInfo.Path}
		mimeOut, _, err := p.ExecCommand(namespace, podName, container, mimeCmd)
		if err == nil && mimeOut != "" {
			fileInfo.MimeType = strings.TrimSpace(mimeOut)
		} else {
			fileInfo.MimeType = p.guessMimeType(fileInfo.Name, false)
		}
	} else if fileInfo.IsDir {
		fileInfo.MimeType = "inode/directory"
	} else {
		fileInfo.MimeType = p.guessMimeType(fileInfo.Name, false)
	}

	return fileInfo, nil
}

// parseStatOutput 解析 stat 命令输出
func (p *podOperator) parseStatOutput(output string, filePath string) (*types.FileInfo, error) {
	output = strings.TrimSpace(output)
	fields := strings.Split(output, "|")
	if len(fields) < 8 {
		return nil, fmt.Errorf("无效的stat输出: %s", output)
	}

	fileInfo := &types.FileInfo{
		Name:  filepath.Base(fields[0]),
		Path:  filePath,
		Owner: fields[4],
		Group: fields[5],
	}

	fileType := strings.ToLower(fields[1])
	fileInfo.IsDir = strings.Contains(fileType, "directory")
	fileInfo.IsLink = strings.Contains(fileType, "symbolic link") || strings.Contains(fileType, "link")

	size, _ := strconv.ParseInt(fields[2], 10, 64)
	fileInfo.Size = size

	timestamp, _ := strconv.ParseInt(fields[3], 10, 64)
	if timestamp > 0 {
		fileInfo.ModTime = time.Unix(timestamp, 0)
	}

	if mode, err := strconv.ParseInt(fields[6], 8, 32); err == nil {
		fileInfo.Mode = fmt.Sprintf("%o", mode)
		fileInfo.Permissions = p.parsePermissions(mode)
		fileInfo.IsReadable = (mode & 0400) != 0
		fileInfo.IsWritable = (mode & 0200) != 0
		fileInfo.IsExecutable = (mode & 0100) != 0
	}

	fileInfo.MimeType = p.guessMimeType(fileInfo.Name, fileInfo.IsDir)

	return fileInfo, nil
}

// ========== 辅助方法 ==========

// filterByFileTypes 按文件类型过滤
func (p *podOperator) filterByFileTypes(files []types.FileInfo, fileTypes []string) []types.FileInfo {
	if len(fileTypes) == 0 {
		return files
	}

	filtered := []types.FileInfo{}
	for _, file := range files {
		// 目录始终保留
		if file.IsDir {
			filtered = append(filtered, file)
			continue
		}
		ext := strings.TrimPrefix(filepath.Ext(file.Name), ".")
		for _, fileType := range fileTypes {
			if strings.EqualFold(ext, fileType) {
				filtered = append(filtered, file)
				break
			}
		}
	}
	return filtered
}

// applyPagination 应用分页
func (p *podOperator) applyPagination(files []types.FileInfo, offset, limit int) []types.FileInfo {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		return files
	}

	start := offset
	if start >= len(files) {
		return []types.FileInfo{}
	}

	end := start + limit
	if end > len(files) {
		end = len(files)
	}

	return files[start:end]
}

// buildFileListResult 构建文件列表结果
func (p *podOperator) buildFileListResult(files []types.FileInfo, dirPath, container string) *types.FileListResult {
	totalSize := int64(0)
	for _, file := range files {
		totalSize += file.Size
	}

	return &types.FileListResult{
		Files:       files,
		CurrentPath: dirPath,
		Breadcrumbs: p.buildBreadcrumbs(dirPath),
		TotalCount:  len(files),
		TotalSize:   totalSize,
		Container:   container,
		HasMore:     false,
	}
}

// filterAndSortFiles 过滤和排序文件列表
func (p *podOperator) filterAndSortFiles(files []types.FileInfo, opts *types.FileListOptions) []types.FileInfo {
	if opts == nil {
		return files
	}

	// 隐藏文件过滤
	if !opts.ShowHidden {
		filtered := []types.FileInfo{}
		for _, file := range files {
			if !strings.HasPrefix(file.Name, ".") {
				filtered = append(filtered, file)
			}
		}
		files = filtered
	}

	// 搜索过滤
	if opts.Search != "" {
		filtered := []types.FileInfo{}
		search := strings.ToLower(opts.Search)
		for _, file := range files {
			if strings.Contains(strings.ToLower(file.Name), search) {
				filtered = append(filtered, file)
			}
		}
		files = filtered
	}

	// 文件类型过滤
	if len(opts.FileTypes) > 0 {
		files = p.filterByFileTypes(files, opts.FileTypes)
	}

	// 排序
	p.sortFiles(files, opts)

	return files
}

// sortFiles 排序文件列表
func (p *podOperator) sortFiles(files []types.FileInfo, opts *types.FileListOptions) {
	if opts == nil {
		return
	}

	sort.Slice(files, func(i, j int) bool {
		var result bool
		switch opts.SortBy {
		case types.SortBySize:
			result = files[i].Size < files[j].Size
		case types.SortByTime:
			result = files[i].ModTime.Before(files[j].ModTime)
		case types.SortByType:
			// 目录优先，然后按名称排序
			if files[i].IsDir != files[j].IsDir {
				result = files[i].IsDir
			} else {
				result = strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
			}
		default: // SortByName
			result = strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
		}

		if opts.SortDesc {
			return !result
		}
		return result
	})
}

// buildBreadcrumbs 构建面包屑导航
func (p *podOperator) buildBreadcrumbs(currentPath string) []types.BreadcrumbItem {
	breadcrumbs := []types.BreadcrumbItem{
		{Name: "Root", Path: "/"},
	}

	if currentPath == "/" || currentPath == "" {
		return breadcrumbs
	}

	// 清理路径
	currentPath = filepath.Clean(currentPath)
	parts := strings.Split(strings.TrimPrefix(currentPath, "/"), "/")
	path := "/"

	for _, part := range parts {
		if part == "" {
			continue
		}
		path = filepath.Join(path, part)
		breadcrumbs = append(breadcrumbs, types.BreadcrumbItem{
			Name: part,
			Path: path,
		})
	}

	return breadcrumbs
}

// guessMimeType 猜测文件 MIME 类型
func (p *podOperator) guessMimeType(filename string, isDir bool) string {
	if isDir {
		return "inode/directory"
	}

	ext := strings.ToLower(filepath.Ext(filename))
	mimeTypes := map[string]string{
		// 文本文件
		".txt":          "text/plain",
		".text":         "text/plain",
		".log":          "text/plain",
		".ini":          "text/plain",
		".cfg":          "text/plain",
		".conf":         "text/plain",
		".config":       "text/plain",
		".properties":   "text/plain",
		".env":          "text/plain",
		".gitignore":    "text/plain",
		".dockerignore": "text/plain",

		// Markdown
		".md":       "text/markdown",
		".markdown": "text/markdown",
		".mdown":    "text/markdown",

		// 数据格式
		".json": "application/json",
		".yaml": "text/yaml",
		".yml":  "text/yaml",
		".xml":  "application/xml",
		".toml": "text/toml",
		".csv":  "text/csv",
		".tsv":  "text/tab-separated-values",

		// Web 前端
		".html":   "text/html",
		".htm":    "text/html",
		".css":    "text/css",
		".scss":   "text/scss",
		".sass":   "text/sass",
		".less":   "text/less",
		".js":     "text/javascript",
		".jsx":    "text/javascript",
		".ts":     "text/typescript",
		".tsx":    "text/typescript",
		".vue":    "text/vue",
		".svelte": "text/svelte",

		// Shell 脚本
		".sh":   "text/x-shellscript",
		".bash": "text/x-shellscript",
		".zsh":  "text/x-shellscript",
		".fish": "text/x-shellscript",
		".ksh":  "text/x-shellscript",
		".csh":  "text/x-shellscript",
		".tcsh": "text/x-shellscript",

		// 编程语言
		".py":     "text/x-python",
		".pyw":    "text/x-python",
		".go":     "text/x-go",
		".java":   "text/x-java",
		".class":  "application/java-vm",
		".jar":    "application/java-archive",
		".c":      "text/x-c",
		".h":      "text/x-c",
		".cpp":    "text/x-c++",
		".cxx":    "text/x-c++",
		".cc":     "text/x-c++",
		".hpp":    "text/x-c++",
		".hxx":    "text/x-c++",
		".cs":     "text/x-csharp",
		".rb":     "text/x-ruby",
		".php":    "text/x-php",
		".pl":     "text/x-perl",
		".pm":     "text/x-perl",
		".swift":  "text/x-swift",
		".kt":     "text/x-kotlin",
		".kts":    "text/x-kotlin",
		".scala":  "text/x-scala",
		".rs":     "text/x-rust",
		".lua":    "text/x-lua",
		".r":      "text/x-r",
		".R":      "text/x-r",
		".m":      "text/x-matlab",
		".sql":    "text/x-sql",
		".groovy": "text/x-groovy",
		".gradle": "text/x-groovy",
		".clj":    "text/x-clojure",
		".erl":    "text/x-erlang",
		".ex":     "text/x-elixir",
		".exs":    "text/x-elixir",
		".hs":     "text/x-haskell",

		// DevOps / 配置
		".dockerfile": "text/x-dockerfile",
		".makefile":   "text/x-makefile",
		".cmake":      "text/x-cmake",
		".tf":         "text/x-terraform",
		".tfvars":     "text/x-terraform",
		".hcl":        "text/x-hcl",
		".nginx":      "text/x-nginx",
		".service":    "text/x-systemd",
		".socket":     "text/x-systemd",
		".timer":      "text/x-systemd",

		// 图片
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".bmp":  "image/bmp",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".ico":  "image/x-icon",
		".tiff": "image/tiff",
		".tif":  "image/tiff",

		// 文档
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".odt":  "application/vnd.oasis.opendocument.text",
		".ods":  "application/vnd.oasis.opendocument.spreadsheet",
		".odp":  "application/vnd.oasis.opendocument.presentation",
		".rtf":  "application/rtf",

		// 压缩文件
		".zip":  "application/zip",
		".tar":  "application/x-tar",
		".gz":   "application/gzip",
		".gzip": "application/gzip",
		".tgz":  "application/gzip",
		".bz2":  "application/x-bzip2",
		".xz":   "application/x-xz",
		".7z":   "application/x-7z-compressed",
		".rar":  "application/x-rar-compressed",
		".lz":   "application/x-lzip",
		".lzma": "application/x-lzma",
		".zst":  "application/zstd",

		// 音视频
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".ogg":  "audio/ogg",
		".flac": "audio/flac",
		".aac":  "audio/aac",
		".m4a":  "audio/mp4",
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".mkv":  "video/x-matroska",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".wmv":  "video/x-ms-wmv",
		".flv":  "video/x-flv",

		// 字体
		".ttf":   "font/ttf",
		".otf":   "font/otf",
		".woff":  "font/woff",
		".woff2": "font/woff2",
		".eot":   "application/vnd.ms-fontobject",

		// 二进制/可执行
		".exe":   "application/x-executable",
		".dll":   "application/x-sharedlib",
		".so":    "application/x-sharedlib",
		".dylib": "application/x-sharedlib",
		".bin":   "application/octet-stream",
		".o":     "application/x-object",
		".a":     "application/x-archive",

		// 证书/密钥
		".pem": "application/x-pem-file",
		".crt": "application/x-x509-ca-cert",
		".cer": "application/x-x509-ca-cert",
		".key": "application/x-pem-file",
		".p12": "application/x-pkcs12",
		".pfx": "application/x-pkcs12",
		".pub": "text/plain",

		// 其他
		".wasm": "application/wasm",
		".map":  "application/json",
		".lock": "text/plain",
		".pid":  "text/plain",
		".sock": "application/octet-stream",
	}

	if mimeType, ok := mimeTypes[ext]; ok {
		return mimeType
	}

	// 处理无扩展名但有特殊文件名的情况
	baseName := strings.ToLower(filepath.Base(filename))
	specialFiles := map[string]string{
		"dockerfile":     "text/x-dockerfile",
		"makefile":       "text/x-makefile",
		"gnumakefile":    "text/x-makefile",
		"cmakelists.txt": "text/x-cmake",
		"rakefile":       "text/x-ruby",
		"gemfile":        "text/x-ruby",
		"procfile":       "text/plain",
		"brewfile":       "text/x-ruby",
		"vagrantfile":    "text/x-ruby",
		"jenkinsfile":    "text/x-groovy",
		".gitconfig":     "text/plain",
		".bashrc":        "text/x-shellscript",
		".bash_profile":  "text/x-shellscript",
		".zshrc":         "text/x-shellscript",
		".profile":       "text/x-shellscript",
		".vimrc":         "text/plain",
		".npmrc":         "text/plain",
		".yarnrc":        "text/plain",
		".editorconfig":  "text/plain",
		"hosts":          "text/plain",
		"passwd":         "text/plain",
		"shadow":         "text/plain",
		"group":          "text/plain",
		"fstab":          "text/plain",
		"crontab":        "text/plain",
		"sudoers":        "text/plain",
	}

	if mimeType, ok := specialFiles[baseName]; ok {
		return mimeType
	}

	return "application/octet-stream"
}

// ========== GetFileStats ==========

// GetFileStats 获取文件或目录的统计信息
func (p *podOperator) GetFileStats(ctx context.Context, namespace, podName, container, path string) (*types.FileStats, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("获取文件统计信息: namespace=%s, pod=%s, container=%s, path=%s",
		namespace, podName, container, path)

	fileInfo, err := p.GetFileInfo(ctx, namespace, podName, container, path)
	if err != nil {
		return nil, err
	}

	stats := &types.FileStats{
		FileInfo: *fileInfo,
	}

	// 探测命令能力
	caps, err := p.detectCommandCapabilities(ctx, namespace, podName, container)
	if err != nil {
		logger.Debugf("无法探测命令能力: %v", err)
		return stats, nil
	}

	if fileInfo.IsDir && caps.hasDu {
		// 获取目录磁盘使用量
		duCmd := []string{"du", "-sb", path}
		duOut, _, err := p.ExecCommand(namespace, podName, container, duCmd)
		if err == nil {
			fields := strings.Fields(duOut)
			if len(fields) > 0 {
				stats.DiskUsage, _ = strconv.ParseInt(fields[0], 10, 64)
			}
		}

		// 统计文件和目录数量
		if caps.hasFind {
			findCmd := []string{"sh", "-c", fmt.Sprintf("find '%s' -maxdepth 1 -type f 2>/dev/null | wc -l", path)}
			fileCountOut, _, _ := p.ExecCommand(namespace, podName, container, findCmd)
			stats.FileCount, _ = strconv.Atoi(strings.TrimSpace(fileCountOut))

			findCmd = []string{"sh", "-c", fmt.Sprintf("find '%s' -maxdepth 1 -type d 2>/dev/null | wc -l", path)}
			dirCountOut, _, _ := p.ExecCommand(namespace, podName, container, findCmd)
			stats.DirCount, _ = strconv.Atoi(strings.TrimSpace(dirCountOut))
			if stats.DirCount > 0 {
				stats.DirCount-- // 减去目录本身
			}
		}
	} else if !fileInfo.IsDir && caps.hasMd5sum {
		// 计算文件 MD5 校验和
		md5Cmd := []string{"md5sum", path}
		md5Out, _, err := p.ExecCommand(namespace, podName, container, md5Cmd)
		if err == nil {
			fields := strings.Fields(md5Out)
			if len(fields) > 0 {
				stats.Checksum = fields[0]
			}
		}
	}

	return stats, nil
}

// ========== 文件下载操作实现 ==========

// DownloadFile 下载文件
func (p *podOperator) DownloadFile(ctx context.Context, namespace, podName, container, filePath string, opts *types.DownloadOptions) (io.ReadCloser, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("开始下载文件: namespace=%s, pod=%s, container=%s, file=%s",
		namespace, podName, container, filePath)

	var err error
	container, err = p.ensureContainer(namespace, podName, container)
	if err != nil {
		return nil, err
	}

	if opts == nil {
		opts = &types.DownloadOptions{}
	}

	// 检查文件是否存在
	testCmd := []string{"test", "-f", filePath}
	_, stderr, err := p.ExecCommand(namespace, podName, container, testCmd)
	if err != nil {
		if pathErr := p.parsePathError(stderr, filePath); pathErr != nil {
			return nil, pathErr
		}
		return nil, fmt.Errorf("文件不存在或无法访问: %s", filePath)
	}

	var cmd []string
	if opts.Compress {
		cmd = []string{"sh", "-c", fmt.Sprintf("cat '%s' | gzip", filePath)}
	} else {
		cmd = []string{"cat", filePath}
	}

	// 支持范围下载
	if opts.RangeStart > 0 || opts.RangeEnd > 0 {
		ddCmd := fmt.Sprintf("dd if='%s' bs=1 skip=%d", filePath, opts.RangeStart)
		if opts.RangeEnd > opts.RangeStart {
			count := opts.RangeEnd - opts.RangeStart + 1
			ddCmd += fmt.Sprintf(" count=%d", count)
		}
		ddCmd += " 2>/dev/null"
		if opts.Compress {
			ddCmd += " | gzip"
		}
		cmd = []string{"sh", "-c", ddCmd}
	}

	pr, pw := io.Pipe()

	executor, err := p.Exec(namespace, podName, container, cmd, types.ExecOptions{
		Stdin:  false,
		Stdout: true,
		Stderr: false,
		TTY:    false,
	})
	if err != nil {
		return nil, fmt.Errorf("创建下载执行器失败: %v", err)
	}

	go func() {
		defer pw.Close()
		err := executor.Stream(remotecommand.StreamOptions{
			Stdin:  nil,
			Stdout: pw,
			Stderr: nil,
			Tty:    false,
		})
		if err != nil {
			logger.Errorf("下载文件失败: %v", err)
		}
	}()

	return pr, nil
}

// DownloadFileChunk 下载文件分块
func (p *podOperator) DownloadFileChunk(ctx context.Context, namespace, podName, container, filePath string, chunkIndex int, chunkSize int64) ([]byte, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("下载文件分块: file=%s, chunk=%d, size=%d", filePath, chunkIndex, chunkSize)

	cmd := []string{"sh", "-c",
		fmt.Sprintf("dd if='%s' bs=%d skip=%d count=1 2>/dev/null", filePath, chunkSize, chunkIndex)}

	stdout, _, err := p.ExecCommand(namespace, podName, container, cmd)
	if err != nil {
		return nil, fmt.Errorf("下载文件分块失败: %v", err)
	}

	return []byte(stdout), nil
}

// ========== 文件上传操作实现 ==========

// UploadFile 上传文件
func (p *podOperator) UploadFile(ctx context.Context, namespace, podName, container, destPath string, reader io.Reader, opts *types.UploadOptions) error {
	logger := logx.WithContext(ctx)
	logger.Infof("开始上传文件: namespace=%s, pod=%s, container=%s, dest=%s",
		namespace, podName, container, destPath)

	var err error
	container, err = p.ensureContainer(namespace, podName, container)
	if err != nil {
		return err
	}

	if opts == nil {
		opts = &types.UploadOptions{
			Overwrite: false,
		}
	}

	// 检查文件是否已存在
	if !opts.Overwrite {
		testCmd := []string{"test", "-e", destPath}
		_, _, err := p.ExecCommand(namespace, podName, container, testCmd)
		if err == nil {
			return fmt.Errorf("文件已存在: %s", destPath)
		}
	}

	// 自动创建目录
	if opts.CreateDirs {
		dirPath := filepath.Dir(destPath)
		mkdirCmd := []string{"mkdir", "-p", dirPath}
		_, _, _ = p.ExecCommand(namespace, podName, container, mkdirCmd)
	}

	// 读取内容
	content, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("读取源文件失败: %v", err)
	}

	maxFileSize := opts.MaxFileSize
	if maxFileSize == 0 {
		maxFileSize = 10 * 1024 * 1024 * 1024 // 10GB
	}

	if int64(len(content)) > maxFileSize {
		return fmt.Errorf("文件大小超过限制 %d MB", maxFileSize/(1024*1024))
	}

	// 使用 tar 方式上传
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	fileName := filepath.Base(destPath)
	if opts.FileName != "" {
		fileName = opts.FileName
	}

	mode := int64(0644)
	if opts.FileMode != "" {
		if parsed, err := strconv.ParseInt(opts.FileMode, 8, 64); err == nil {
			mode = parsed
		}
	}

	hdr := &tar.Header{
		Name:    fileName,
		Mode:    mode,
		Size:    int64(len(content)),
		ModTime: time.Now(),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("写入tar header失败: %v", err)
	}

	if _, err := tw.Write(content); err != nil {
		return fmt.Errorf("写入tar内容失败: %v", err)
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("关闭tar writer失败: %v", err)
	}

	// 通过 tar 解压到目标位置
	destDir := filepath.Dir(destPath)
	cmd := []string{"tar", "-xf", "-", "-C", destDir}

	var stderrBuf bytes.Buffer
	streams := types.IOStreams{
		In:     &buf,
		Out:    io.Discard,
		ErrOut: &stderrBuf,
	}

	err = p.ExecWithStreams(namespace, podName, container, cmd, streams)
	if err != nil {
		return fmt.Errorf("上传文件失败: %v, stderr=%s", err, stderrBuf.String())
	}

	// 验证文件
	if opts.Checksum != "" {
		md5Cmd := []string{"md5sum", destPath}
		md5Out, _, err := p.ExecCommand(namespace, podName, container, md5Cmd)
		if err == nil {
			fields := strings.Fields(md5Out)
			if len(fields) > 0 && fields[0] != opts.Checksum {
				p.ExecCommand(namespace, podName, container, []string{"rm", "-f", destPath})
				return fmt.Errorf("文件校验和不匹配")
			}
		}
	}

	return nil
}

// ========== 文件管理操作实现 ==========

// CreateDirectory 创建目录
func (p *podOperator) CreateDirectory(ctx context.Context, namespace, podName, container, dirPath string, opts *types.CreateDirOptions) error {
	logger := logx.WithContext(ctx)
	logger.Infof("创建目录: namespace=%s, pod=%s, container=%s, dir=%s",
		namespace, podName, container, dirPath)

	var err error
	container, err = p.ensureContainer(namespace, podName, container)
	if err != nil {
		return err
	}

	if opts == nil {
		opts = &types.CreateDirOptions{
			Mode:          DefaultDirMode,
			CreateParents: true,
		}
	}

	cmd := []string{"mkdir"}
	if opts.CreateParents {
		cmd = append(cmd, "-p")
	}
	if opts.Mode != "" {
		cmd = append(cmd, "-m", opts.Mode)
	}
	cmd = append(cmd, dirPath)

	_, stderr, err := p.ExecCommand(namespace, podName, container, cmd)
	if err != nil {
		if pathErr := p.parsePathError(stderr, dirPath); pathErr != nil {
			return pathErr
		}
		return fmt.Errorf("创建目录失败: %v", err)
	}

	return nil
}

// DeleteFiles 删除文件或目录
func (p *podOperator) DeleteFiles(ctx context.Context, namespace, podName, container string, paths []string, opts *types.DeleteOptions) (*types.DeleteResult, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("删除文件: namespace=%s, pod=%s, container=%s, 数量=%d",
		namespace, podName, container, len(paths))

	var err error
	container, err = p.ensureContainer(namespace, podName, container)
	if err != nil {
		return nil, err
	}

	if opts == nil {
		opts = &types.DeleteOptions{
			Recursive: false,
			Force:     false,
		}
	}

	result := &types.DeleteResult{
		DeletedCount: 0,
		FailedPaths:  []string{},
		Errors:       []error{},
	}

	for _, filePath := range paths {
		cmd := []string{"rm"}
		if opts.Recursive {
			cmd = append(cmd, "-r")
		}
		if opts.Force {
			cmd = append(cmd, "-f")
		}
		cmd = append(cmd, filePath)

		_, stderr, err := p.ExecCommand(namespace, podName, container, cmd)
		if err != nil {
			logger.Errorf("删除失败: path=%s, err=%v, stderr=%s", filePath, err, stderr)
			result.FailedPaths = append(result.FailedPaths, filePath)

			errMsg := fmt.Sprintf("删除 %s 失败", filePath)
			if pathErr := p.parsePathError(stderr, filePath); pathErr != nil {
				errMsg = pathErr.Error()
			}
			result.Errors = append(result.Errors, fmt.Errorf(errMsg))
		} else {
			result.DeletedCount++
		}
	}

	return result, nil
}

// MoveFile 移动或重命名文件
func (p *podOperator) MoveFile(ctx context.Context, namespace, podName, container, sourcePath, destPath string, overwrite bool) error {
	logger := logx.WithContext(ctx)
	logger.Infof("移动文件: %s -> %s", sourcePath, destPath)

	var err error
	container, err = p.ensureContainer(namespace, podName, container)
	if err != nil {
		return err
	}

	// 检查源文件
	testCmd := []string{"test", "-e", sourcePath}
	_, stderr, err := p.ExecCommand(namespace, podName, container, testCmd)
	if err != nil {
		return fmt.Errorf("源路径不存在: %s", sourcePath)
	}

	// 检查目标文件
	if !overwrite {
		testCmd = []string{"test", "-e", destPath}
		_, _, err = p.ExecCommand(namespace, podName, container, testCmd)
		if err == nil {
			return fmt.Errorf("目标路径已存在: %s", destPath)
		}
	}

	cmd := []string{"mv"}
	if overwrite {
		cmd = append(cmd, "-f")
	}
	cmd = append(cmd, sourcePath, destPath)

	_, stderr, err = p.ExecCommand(namespace, podName, container, cmd)
	if err != nil {
		if pathErr := p.parsePathError(stderr, sourcePath); pathErr != nil {
			return pathErr
		}
		return fmt.Errorf("移动文件失败: %v", err)
	}

	return nil
}

// CopyFile 复制文件或目录
func (p *podOperator) CopyFile(ctx context.Context, namespace, podName, container, sourcePath, destPath string, opts *types.CopyOptions) error {
	logger := logx.WithContext(ctx)
	logger.Infof("复制文件: %s -> %s", sourcePath, destPath)

	var err error
	container, err = p.ensureContainer(namespace, podName, container)
	if err != nil {
		return err
	}

	if opts == nil {
		opts = &types.CopyOptions{
			Overwrite:     false,
			Recursive:     false,
			PreserveAttrs: true,
		}
	}

	// 检查源文件
	testCmd := []string{"test", "-e", sourcePath}
	_, stderr, err := p.ExecCommand(namespace, podName, container, testCmd)
	if err != nil {
		return fmt.Errorf("源路径不存在: %s", sourcePath)
	}

	// 检查目标文件
	if !opts.Overwrite {
		testCmd = []string{"test", "-e", destPath}
		_, _, err = p.ExecCommand(namespace, podName, container, testCmd)
		if err == nil {
			return fmt.Errorf("目标路径已存在: %s", destPath)
		}
	}

	cmd := []string{"cp"}
	if opts.Recursive {
		cmd = append(cmd, "-r")
	}
	if opts.PreserveAttrs {
		cmd = append(cmd, "-p")
	}
	if opts.Overwrite {
		cmd = append(cmd, "-f")
	}
	cmd = append(cmd, sourcePath, destPath)

	_, stderr, err = p.ExecCommand(namespace, podName, container, cmd)
	if err != nil {
		if pathErr := p.parsePathError(stderr, sourcePath); pathErr != nil {
			return pathErr
		}
		return fmt.Errorf("复制文件失败: %v", err)
	}

	return nil
}

// ========== 文件内容操作实现 ==========

// ReadFile 读取文件内容
func (p *podOperator) ReadFile(ctx context.Context, namespace, podName, container, filePath string, opts *types.ReadOptions) (*types.FileContent, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("读取文件内容: file=%s", filePath)

	var err error
	container, err = p.ensureContainer(namespace, podName, container)
	if err != nil {
		return nil, err
	}

	if opts == nil {
		opts = &types.ReadOptions{
			Limit:    10 * 1024 * 1024,
			Encoding: "UTF-8",
		}
	}

	// 获取文件大小
	var fileSize int64
	statCmd := []string{"stat", "-c", "%s", filePath}
	sizeOut, stderr, err := p.ExecCommand(namespace, podName, container, statCmd)
	if err != nil {
		// 尝试 GNU stat
		statCmd = []string{"stat", "--format", "%s", filePath}
		sizeOut, stderr, err = p.ExecCommand(namespace, podName, container, statCmd)
		if err != nil {
			if pathErr := p.parsePathError(stderr, filePath); pathErr != nil {
				return nil, pathErr
			}
			return nil, fmt.Errorf("获取文件大小失败: %v", err)
		}
	}
	fileSize, _ = strconv.ParseInt(strings.TrimSpace(sizeOut), 10, 64)

	var cmd []string
	if opts.Tail && opts.TailLines > 0 {
		cmd = []string{"tail", "-n", strconv.Itoa(opts.TailLines), filePath}
	} else if opts.Offset > 0 || opts.Limit > 0 {
		ddCmd := fmt.Sprintf("dd if='%s' bs=1", filePath)
		if opts.Offset > 0 {
			ddCmd += fmt.Sprintf(" skip=%d", opts.Offset)
		}
		if opts.Limit > 0 {
			ddCmd += fmt.Sprintf(" count=%d", opts.Limit)
		}
		ddCmd += " 2>/dev/null"
		cmd = []string{"sh", "-c", ddCmd}
	} else {
		cmd = []string{"cat", filePath}
	}

	stdout, stderr, err := p.ExecCommand(namespace, podName, container, cmd)
	if err != nil {
		if pathErr := p.parsePathError(stderr, filePath); pathErr != nil {
			return nil, pathErr
		}
		return nil, fmt.Errorf("读取文件内容失败: %v", err)
	}

	// 检测是否为文本文件
	isText := true
	mimeType := p.guessMimeType(filePath, false)
	if strings.HasPrefix(mimeType, "text/") ||
		mimeType == "application/json" ||
		mimeType == "application/xml" ||
		strings.Contains(mimeType, "yaml") {
		isText = true
	} else if strings.HasPrefix(mimeType, "image/") ||
		strings.HasPrefix(mimeType, "video/") ||
		strings.HasPrefix(mimeType, "audio/") ||
		mimeType == "application/octet-stream" {
		isText = false
	}

	lineCount := 0
	if isText {
		lineCount = strings.Count(stdout, "\n")
		if len(stdout) > 0 && !strings.HasSuffix(stdout, "\n") {
			lineCount++
		}
	}

	content := &types.FileContent{
		Content:     stdout,
		BinaryData:  []byte(stdout),
		FileSize:    fileSize,
		BytesRead:   int64(len(stdout)),
		IsText:      isText,
		Encoding:    opts.Encoding,
		IsTruncated: opts.Limit > 0 && fileSize > opts.Limit,
		LineCount:   lineCount,
	}

	return content, nil
}

// SaveFile 保存文件内容
func (p *podOperator) SaveFile(ctx context.Context, namespace, podName, container, filePath string, content []byte, opts *types.SaveOptions) error {
	logger := logx.WithContext(ctx)
	logger.Infof("保存文件内容: file=%s, size=%d bytes", filePath, len(content))

	var err error
	container, err = p.ensureContainer(namespace, podName, container)
	if err != nil {
		return err
	}

	if opts == nil {
		opts = &types.SaveOptions{
			Encoding: "UTF-8",
			FileMode: DefaultFileMode,
		}
	}

	if len(content) > MaxFileSize {
		return fmt.Errorf("文件大小超过限制 %d GB", MaxFileSize/(1024*1024*1024))
	}

	// 检查文件是否存在
	testCmd := []string{"test", "-f", filePath}
	_, _, fileExistsErr := p.ExecCommand(namespace, podName, container, testCmd)
	fileExists := fileExistsErr == nil

	if !fileExists && !opts.CreateIfNotExists {
		return fmt.Errorf("文件不存在: %s", filePath)
	}

	// 备份文件
	if opts.Backup && fileExists {
		backupPath := fmt.Sprintf("%s.bak.%d", filePath, time.Now().Unix())
		cpCmd := []string{"cp", "-p", filePath, backupPath}
		p.ExecCommand(namespace, podName, container, cpCmd)
	}

	// 确保目录存在
	dirPath := filepath.Dir(filePath)
	if dirPath != "/" && dirPath != "." {
		mkdirCmd := []string{"mkdir", "-p", dirPath}
		p.ExecCommand(namespace, podName, container, mkdirCmd)
	}

	// 使用 tar 方式上传
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	fileName := filepath.Base(filePath)
	mode := int64(0644)
	if opts.FileMode != "" {
		if parsed, err := strconv.ParseInt(opts.FileMode, 8, 64); err == nil {
			mode = parsed
		}
	}

	hdr := &tar.Header{
		Name:    fileName,
		Mode:    mode,
		Size:    int64(len(content)),
		ModTime: time.Now(),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("写入tar header失败: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		return fmt.Errorf("写入tar内容失败: %v", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("关闭tar writer失败: %v", err)
	}

	cmd := []string{"tar", "-xf", "-", "-C", dirPath}
	var stderrBuf bytes.Buffer
	streams := types.IOStreams{
		In:     &buf,
		Out:    io.Discard,
		ErrOut: &stderrBuf,
	}

	err = p.ExecWithStreams(namespace, podName, container, cmd, streams)
	if err != nil {
		return fmt.Errorf("保存文件失败: %v, stderr=%s", err, stderrBuf.String())
	}

	// 设置权限
	if opts.FileMode != "" {
		chmodCmd := []string{"chmod", opts.FileMode, filePath}
		p.ExecCommand(namespace, podName, container, chmodCmd)
	}

	return nil
}

// TailFile 实时跟踪文件内容
func (p *podOperator) TailFile(ctx context.Context, namespace, podName, container, filePath string, opts *types.TailOptions) (chan string, chan error, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("开始跟踪文件: file=%s", filePath)

	var err error
	container, err = p.ensureContainer(namespace, podName, container)
	if err != nil {
		return nil, nil, err
	}

	if opts == nil {
		opts = &types.TailOptions{
			Lines:     10,
			Follow:    true,
			MaxBuffer: 100,
		}
	}

	lineChan := make(chan string, opts.MaxBuffer)
	errChan := make(chan error, 1)

	cmd := []string{"tail"}
	if opts.Lines > 0 {
		cmd = append(cmd, "-n", strconv.Itoa(opts.Lines))
	}
	if opts.Follow {
		cmd = append(cmd, "-f")
	}
	if opts.Retry {
		cmd = append(cmd, "--retry")
	}
	cmd = append(cmd, filePath)

	executor, err := p.Exec(namespace, podName, container, cmd, types.ExecOptions{
		Stdin:  false,
		Stdout: true,
		Stderr: true,
		TTY:    false,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("创建执行器失败: %v", err)
	}

	go func() {
		defer close(lineChan)
		defer close(errChan)

		pr, pw := io.Pipe()
		defer pr.Close()
		defer pw.Close()

		go func() {
			streamErr := executor.Stream(remotecommand.StreamOptions{
				Stdin:  nil,
				Stdout: pw,
				Stderr: pw,
				Tty:    false,
			})
			if streamErr != nil {
				select {
				case errChan <- streamErr:
				default:
				}
			}
			pw.Close()
		}()

		scanner := bufio.NewScanner(pr)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if scanner.Scan() {
					select {
					case lineChan <- scanner.Text():
					case <-ctx.Done():
						return
					}
				} else {
					if err := scanner.Err(); err != nil {
						select {
						case errChan <- err:
						default:
						}
					}
					return
				}
			}
		}
	}()

	return lineChan, errChan, nil
}

// ========== 辅助功能实现 ==========

// CheckFilePermission 检查文件权限
func (p *podOperator) CheckFilePermission(ctx context.Context, namespace, podName, container, path string, permission types.FilePermission) (bool, error) {
	fileInfo, err := p.GetFileInfo(ctx, namespace, podName, container, path)
	if err != nil {
		return false, err
	}

	if permission.User.Read && !fileInfo.Permissions.User.Read {
		return false, nil
	}
	if permission.User.Write && !fileInfo.Permissions.User.Write {
		return false, nil
	}
	if permission.User.Execute && !fileInfo.Permissions.User.Execute {
		return false, nil
	}

	return true, nil
}

// CompressFiles 压缩文件或目录
func (p *podOperator) CompressFiles(ctx context.Context, namespace, podName, container string, paths []string, destPath string, format types.CompressionFormat) error {
	logger := logx.WithContext(ctx)
	logger.Infof("压缩文件: format=%s, 文件数=%d", format, len(paths))

	var err error
	container, err = p.ensureContainer(namespace, podName, container)
	if err != nil {
		return err
	}

	// 转义路径中的特殊字符
	quotedPaths := make([]string, len(paths))
	for i, p := range paths {
		quotedPaths[i] = fmt.Sprintf("'%s'", p)
	}

	var cmd []string
	switch format {
	case types.CompressionGzip:
		if len(paths) == 1 {
			cmd = []string{"sh", "-c", fmt.Sprintf("gzip -c %s > '%s'", quotedPaths[0], destPath)}
		} else {
			fileList := strings.Join(quotedPaths, " ")
			cmd = []string{"sh", "-c", fmt.Sprintf("tar -czf '%s' %s", destPath, fileList)}
		}
	case types.CompressionZip:
		cmd = append([]string{"zip", "-r", destPath}, paths...)
	case types.CompressionTar:
		cmd = append([]string{"tar", "-cf", destPath}, paths...)
	case types.CompressionTgz:
		cmd = append([]string{"tar", "-czf", destPath}, paths...)
	default:
		return fmt.Errorf("不支持的压缩格式: %s", format)
	}

	_, stderr, err := p.ExecCommand(namespace, podName, container, cmd)
	if err != nil {
		return fmt.Errorf("压缩文件失败: %v, stderr=%s", err, stderr)
	}

	return nil
}

// ExtractArchive 解压文件
func (p *podOperator) ExtractArchive(ctx context.Context, namespace, podName, container, archivePath, destPath string) error {
	logger := logx.WithContext(ctx)
	logger.Infof("解压文件: archive=%s -> dest=%s", archivePath, destPath)

	var err error
	container, err = p.ensureContainer(namespace, podName, container)
	if err != nil {
		return err
	}

	// 检查归档文件
	testCmd := []string{"test", "-f", archivePath}
	_, stderr, err := p.ExecCommand(namespace, podName, container, testCmd)
	if err != nil {
		if pathErr := p.parsePathError(stderr, archivePath); pathErr != nil {
			return pathErr
		}
		return fmt.Errorf("归档文件不存在: %s", archivePath)
	}

	// 创建目标目录
	mkdirCmd := []string{"mkdir", "-p", destPath}
	p.ExecCommand(namespace, podName, container, mkdirCmd)

	var cmd []string
	archiveLower := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(archiveLower, ".tar.gz") || strings.HasSuffix(archiveLower, ".tgz"):
		cmd = []string{"tar", "-xzf", archivePath, "-C", destPath}
	case strings.HasSuffix(archiveLower, ".tar.bz2") || strings.HasSuffix(archiveLower, ".tbz2"):
		cmd = []string{"tar", "-xjf", archivePath, "-C", destPath}
	case strings.HasSuffix(archiveLower, ".tar.xz") || strings.HasSuffix(archiveLower, ".txz"):
		cmd = []string{"tar", "-xJf", archivePath, "-C", destPath}
	case strings.HasSuffix(archiveLower, ".tar"):
		cmd = []string{"tar", "-xf", archivePath, "-C", destPath}
	case strings.HasSuffix(archiveLower, ".gz"):
		outputFile := filepath.Join(destPath, strings.TrimSuffix(filepath.Base(archivePath), ".gz"))
		cmd = []string{"sh", "-c", fmt.Sprintf("gunzip -c '%s' > '%s'", archivePath, outputFile)}
	case strings.HasSuffix(archiveLower, ".zip"):
		cmd = []string{"unzip", "-o", archivePath, "-d", destPath}
	default:
		return fmt.Errorf("不支持的归档格式: %s", archivePath)
	}

	_, stderr, err = p.ExecCommand(namespace, podName, container, cmd)
	if err != nil {
		return fmt.Errorf("解压文件失败: %v, stderr=%s", err, stderr)
	}

	return nil
}

// ========== 文件搜索操作 ==========

// SearchFiles 搜索文件（带降级方案）
func (p *podOperator) SearchFiles(ctx context.Context, namespace, podName, container, searchPath string, opts *types.FileSearchOptions) (*types.FileSearchResponse, error) {
	logger := logx.WithContext(ctx)
	logger.Infof("开始搜索文件: path=%s, pattern=%s", searchPath, opts.Pattern)

	var err error
	container, err = p.ensureContainer(namespace, podName, container)
	if err != nil {
		return nil, err
	}

	if searchPath == "" {
		searchPath = "/"
	}

	if opts == nil {
		opts = &types.FileSearchOptions{
			MaxResults: 100,
			MaxDepth:   10,
		}
	}

	if opts.MaxResults == 0 {
		opts.MaxResults = 100
	}
	if opts.MaxDepth == 0 {
		opts.MaxDepth = 10
	}

	startTime := time.Now()

	// 探测命令能力
	caps, err := p.detectCommandCapabilities(ctx, namespace, podName, container)
	if err != nil {
		logger.Debugf("无法探测命令能力: %v", err)
		caps = &commandCapabilities{hasLs: true}
	}

	var results []types.FileSearchResult

	// 使用 find 或降级到 ls
	if caps.hasFind {
		logger.Debug("使用 find 命令搜索")
		results, err = p.searchWithFind(ctx, namespace, podName, container, searchPath, opts)
	} else if caps.hasLs {
		logger.Debug("find 不可用，降级使用 ls")
		results, err = p.searchWithLs(ctx, namespace, podName, container, searchPath, opts)
	} else {
		return nil, fmt.Errorf("容器中缺少必要的文件搜索命令")
	}

	if err != nil {
		if pathErr := p.parsePathError(err.Error(), searchPath); pathErr != nil {
			return nil, pathErr
		}
		return nil, fmt.Errorf("搜索文件失败: %v", err)
	}

	// 如果需要内容搜索
	if opts.ContentSearch != "" && caps.hasGrep {
		logger.Infof("执行内容搜索: keyword=%s", opts.ContentSearch)
		results = p.performContentSearch(ctx, namespace, podName, container, results, opts)
	}

	// 限制结果数量
	truncated := false
	if len(results) > opts.MaxResults {
		results = results[:opts.MaxResults]
		truncated = true
	}

	searchTime := time.Since(startTime).Milliseconds()

	response := &types.FileSearchResponse{
		Results:    results,
		TotalFound: len(results),
		SearchTime: searchTime,
		Truncated:  truncated,
	}

	return response, nil
}

// searchWithFind 使用 find 命令搜索
func (p *podOperator) searchWithFind(ctx context.Context, namespace, podName, container, searchPath string, opts *types.FileSearchOptions) ([]types.FileSearchResult, error) {
	findCmd := p.buildSearchCommand(searchPath, opts)
	stdout, stderr, err := p.ExecCommand(namespace, podName, container, findCmd)
	if stdout == "" && err != nil {
		return nil, fmt.Errorf("find 失败: %v, stderr=%s", err, stderr)
	}

	return p.parseSearchResults(stdout, opts)
}

// searchWithLs 使用 ls 递归搜索
func (p *podOperator) searchWithLs(ctx context.Context, namespace, podName, container, searchPath string, opts *types.FileSearchOptions) ([]types.FileSearchResult, error) {
	results := []types.FileSearchResult{}
	visited := make(map[string]bool)

	err := p.recursiveSearchWithLs(ctx, namespace, podName, container, searchPath, searchPath, 0, opts, &results, visited)
	if err != nil {
		return nil, err
	}

	// 过滤
	if opts.Pattern != "" {
		results = p.filterByPattern(results, opts.Pattern, opts.CaseSensitive)
	}
	results = p.filterBySize(results, opts.MinSize, opts.MaxSize)
	results = p.filterByTime(results, opts.ModifiedAfter, opts.ModifiedBefore)
	if len(opts.FileTypes) > 0 {
		results = p.filterBySearchFileTypes(results, opts.FileTypes)
	}

	return results, nil
}

// buildSearchCommand 构建搜索命令
func (p *podOperator) buildSearchCommand(searchPath string, opts *types.FileSearchOptions) []string {
	findExpr := fmt.Sprintf("find '%s'", searchPath)

	if opts.MaxDepth > 0 {
		findExpr += fmt.Sprintf(" -maxdepth %d", opts.MaxDepth)
	}

	if opts.Pattern != "" {
		if opts.CaseSensitive {
			findExpr += fmt.Sprintf(" -name '%s'", opts.Pattern)
		} else {
			findExpr += fmt.Sprintf(" -iname '%s'", opts.Pattern)
		}
	}

	if len(opts.FileTypes) > 0 {
		findExpr += " \\("
		for i, ext := range opts.FileTypes {
			if i > 0 {
				findExpr += " -o"
			}
			if opts.CaseSensitive {
				findExpr += fmt.Sprintf(" -name '*.%s'", ext)
			} else {
				findExpr += fmt.Sprintf(" -iname '*.%s'", ext)
			}
		}
		findExpr += " \\)"
	}

	if opts.MinSize > 0 {
		findExpr += fmt.Sprintf(" -size +%dc", opts.MinSize)
	}
	if opts.MaxSize > 0 {
		findExpr += fmt.Sprintf(" -size -%dc", opts.MaxSize)
	}

	if opts.ModifiedAfter != nil {
		days := int(time.Since(*opts.ModifiedAfter).Hours() / 24)
		if days >= 0 {
			findExpr += fmt.Sprintf(" -mtime -%d", days+1)
		}
	}
	if opts.ModifiedBefore != nil {
		days := int(time.Since(*opts.ModifiedBefore).Hours() / 24)
		if days >= 0 {
			findExpr += fmt.Sprintf(" -mtime +%d", days-1)
		}
	}

	if !opts.FollowLinks {
		findExpr += " ! -type l"
	}

	findExpr += " -printf '%p|%s|%T@|%y\\n'"

	if opts.MaxResults > 0 {
		findExpr += fmt.Sprintf(" 2>/dev/null | head -n %d", opts.MaxResults)
	} else {
		findExpr += " 2>/dev/null"
	}

	return []string{"sh", "-c", findExpr}
}

// parseSearchResults 解析搜索结果
func (p *podOperator) parseSearchResults(output string, opts *types.FileSearchOptions) ([]types.FileSearchResult, error) {
	results := []types.FileSearchResult{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, "|")
		if len(fields) < 4 {
			continue
		}

		path := fields[0]
		name := filepath.Base(path)

		size, _ := strconv.ParseInt(fields[1], 10, 64)
		timestamp, _ := strconv.ParseFloat(fields[2], 64)
		fileType := fields[3]

		isDir := fileType == "d"

		var modTime time.Time
		if timestamp > 0 {
			modTime = time.Unix(int64(timestamp), 0)
		}

		result := types.FileSearchResult{
			Path:    path,
			Name:    name,
			Size:    size,
			ModTime: modTime,
			IsDir:   isDir,
			Matches: []types.ContentMatch{},
		}

		results = append(results, result)
	}

	return results, nil
}

// recursiveSearchWithLs 使用 ls 递归搜索
func (p *podOperator) recursiveSearchWithLs(ctx context.Context, namespace, podName, container, currentPath, basePath string, depth int, opts *types.FileSearchOptions, results *[]types.FileSearchResult, visited map[string]bool) error {
	if opts.MaxDepth > 0 && depth > opts.MaxDepth {
		return nil
	}

	if len(*results) >= opts.MaxResults {
		return nil
	}

	if visited[currentPath] {
		return nil
	}
	visited[currentPath] = true

	lsCmd := []string{"ls", "-la", currentPath}
	stdout, _, err := p.ExecCommand(namespace, podName, container, lsCmd)
	if err != nil {
		return nil // 忽略无法访问的目录
	}

	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}

		name := strings.Join(fields[8:], " ")

		// 处理符号链接
		if strings.Contains(name, " -> ") {
			parts := strings.SplitN(name, " -> ", 2)
			name = parts[0]
		}

		if name == "." || name == ".." {
			continue
		}

		fullPath := filepath.Join(currentPath, name)
		isDir := strings.HasPrefix(fields[0], "d")
		isLink := strings.HasPrefix(fields[0], "l")

		if isLink && !opts.FollowLinks {
			continue
		}

		size, _ := strconv.ParseInt(fields[4], 10, 64)
		timeStr := fmt.Sprintf("%s %s %s", fields[5], fields[6], fields[7])
		modTime := p.parseBusyBoxTime(timeStr)

		result := types.FileSearchResult{
			Path:    fullPath,
			Name:    name,
			Size:    size,
			ModTime: modTime,
			IsDir:   isDir,
			Matches: []types.ContentMatch{},
		}

		*results = append(*results, result)

		if isDir && depth < opts.MaxDepth {
			err := p.recursiveSearchWithLs(ctx, namespace, podName, container, fullPath, basePath, depth+1, opts, results, visited)
			if err != nil {
				return err
			}
		}

		if len(*results) >= opts.MaxResults {
			return nil
		}
	}

	return nil
}

// 搜索过滤方法
func (p *podOperator) filterByPattern(results []types.FileSearchResult, pattern string, caseSensitive bool) []types.FileSearchResult {
	filtered := []types.FileSearchResult{}
	for _, result := range results {
		var matched bool
		if caseSensitive {
			matched, _ = filepath.Match(pattern, result.Name)
		} else {
			matched, _ = filepath.Match(strings.ToLower(pattern), strings.ToLower(result.Name))
		}
		if matched {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

func (p *podOperator) filterBySize(results []types.FileSearchResult, minSize, maxSize int64) []types.FileSearchResult {
	if minSize == 0 && maxSize == 0 {
		return results
	}
	filtered := []types.FileSearchResult{}
	for _, result := range results {
		if result.IsDir {
			filtered = append(filtered, result)
			continue
		}
		if minSize > 0 && result.Size < minSize {
			continue
		}
		if maxSize > 0 && result.Size > maxSize {
			continue
		}
		filtered = append(filtered, result)
	}
	return filtered
}

func (p *podOperator) filterByTime(results []types.FileSearchResult, after, before *time.Time) []types.FileSearchResult {
	if after == nil && before == nil {
		return results
	}
	filtered := []types.FileSearchResult{}
	for _, result := range results {
		if after != nil && result.ModTime.Before(*after) {
			continue
		}
		if before != nil && result.ModTime.After(*before) {
			continue
		}
		filtered = append(filtered, result)
	}
	return filtered
}

func (p *podOperator) filterBySearchFileTypes(results []types.FileSearchResult, fileTypes []string) []types.FileSearchResult {
	if len(fileTypes) == 0 {
		return results
	}
	filtered := []types.FileSearchResult{}
	for _, result := range results {
		if result.IsDir {
			filtered = append(filtered, result)
			continue
		}
		ext := strings.TrimPrefix(filepath.Ext(result.Name), ".")
		for _, fileType := range fileTypes {
			if strings.EqualFold(ext, fileType) {
				filtered = append(filtered, result)
				break
			}
		}
	}
	return filtered
}

// performContentSearch 执行内容搜索
func (p *podOperator) performContentSearch(ctx context.Context, namespace, podName, container string, results []types.FileSearchResult, opts *types.FileSearchOptions) []types.FileSearchResult {
	logger := logx.WithContext(ctx)
	filtered := []types.FileSearchResult{}

	maxFiles := 50
	if len(results) > maxFiles {
		results = results[:maxFiles]
	}

	for _, result := range results {
		if result.IsDir {
			continue
		}
		if result.Size > 10*1024*1024 {
			continue
		}
		if p.isBinaryFile(result.Path) {
			continue
		}

		grepCmd := []string{"grep"}
		if !opts.CaseSensitive {
			grepCmd = append(grepCmd, "-i")
		}
		grepCmd = append(grepCmd, "-n", opts.ContentSearch, result.Path)

		stdout, _, err := p.ExecCommand(namespace, podName, container, grepCmd)
		if err != nil {
			continue
		}

		matches := p.parseGrepOutput(stdout)
		if len(matches) > 0 {
			result.Matches = matches
			filtered = append(filtered, result)
			logger.Debugf("文件 %s 找到 %d 个匹配", result.Path, len(matches))
		}
	}

	return filtered
}

// parseGrepOutput 解析 grep 输出
func (p *podOperator) parseGrepOutput(output string) []types.ContentMatch {
	matches := []types.ContentMatch{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}

		lineNum, err := strconv.Atoi(line[:idx])
		if err != nil {
			continue
		}

		content := line[idx+1:]
		match := types.ContentMatch{
			LineNumber: lineNum,
			Line:       content,
			Preview:    content,
		}

		if len(match.Preview) > 200 {
			match.Preview = match.Preview[:200] + "..."
		}

		matches = append(matches, match)
	}

	return matches
}

// isBinaryFile 检查是否为二进制文件
func (p *podOperator) isBinaryFile(path string) bool {
	binaryExts := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".bin": true, ".o": true, ".a": true,
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".pdf": true, ".bmp": true,
		".zip": true, ".tar": true, ".gz": true, ".mp3": true, ".mp4": true, ".avi": true,
		".class": true, ".jar": true, ".war": true, ".ear": true,
		".ico": true, ".webp": true, ".tiff": true, ".tif": true,
		".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true,
	}

	ext := strings.ToLower(filepath.Ext(path))
	return binaryExts[ext]
}

// ensureContainerWithCache 确保容器名称有效
func (p *podOperator) ensureContainerWithCache(namespace, podName, container string) (string, error) {
	if container != "" {
		var pod *corev1.Pod
		var err error

		if p.useInformer && p.podLister != nil {
			pod, err = p.podLister.Pods(namespace).Get(podName)
		} else {
			pod, err = p.client.CoreV1().Pods(namespace).Get(p.ctx, podName, metav1.GetOptions{})
		}

		if err != nil {
			return "", fmt.Errorf("获取Pod失败: %v", err)
		}

		for _, c := range pod.Spec.Containers {
			if c.Name == container {
				return container, nil
			}
		}
		for _, c := range pod.Spec.InitContainers {
			if c.Name == container {
				return container, nil
			}
		}
		for _, c := range pod.Spec.EphemeralContainers {
			if c.Name == container {
				return container, nil
			}
		}

		return "", fmt.Errorf("容器 %s 不存在", container)
	}

	return p.GetDefaultContainer(namespace, podName)
}
