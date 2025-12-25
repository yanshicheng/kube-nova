// common/uploadcore/service.go
package uploadcore

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	DefaultMaxFileSize    = 10 * 1024 * 1024 * 1024 // 10GB
	DefaultMaxChunkSize   = 10 * 1024 * 1024        // 10MB
	DefaultChunkSize      = 1 * 1024 * 1024         // 1MB
	DefaultSessionTimeout = 2 * time.Hour
	CleanupInterval       = 5 * time.Minute
)

// Service 通用上传服务
type Service struct {
	sessions      map[string]*sessionWrapper
	mu            sync.RWMutex
	localCacheDir string
	ctx           context.Context
	cancel        context.CancelFunc
}

type sessionWrapper struct {
	session  *UploadSession
	tempFile *os.File
	mu       sync.Mutex
}

var (
	globalService     *Service
	globalServiceOnce sync.Once
)

// InitService 初始化上传服务
func InitService(localCacheDir string) *Service {
	globalServiceOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())

		globalService = &Service{
			sessions:      make(map[string]*sessionWrapper),
			localCacheDir: localCacheDir,
			ctx:           ctx,
			cancel:        cancel,
		}

		tempDir := globalService.getTempDir()
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			logx.Errorf("创建临时目录失败: %v", err)
		}

		go globalService.startCleanupTask()
		globalService.registerShutdown()

		logx.Infof("通用上传服务初始化成功，临时目录: %s", tempDir)
	})

	return globalService
}

// GetService 获取上传服务实例
func GetService() *Service {
	if globalService == nil {
		panic("upload service not initialized, call InitService first")
	}
	return globalService
}

func (s *Service) getTempDir() string {
	dateDir := time.Now().Format("20060102")
	return filepath.Join(s.localCacheDir, "upload_temp", dateDir)
}

// Init 初始化上传（通用）
func (s *Service) Init(sessionID, uploadType, fileName string, fileSize, chunkSize int64, maxFileSize int64, metadata map[string]interface{}) (*UploadSession, error) {
	// 验证文件大小
	if fileSize <= 0 {
		return nil, fmt.Errorf("文件大小必须大于 0")
	}
	if maxFileSize == 0 {
		maxFileSize = DefaultMaxFileSize
	}
	if fileSize > maxFileSize {
		return nil, fmt.Errorf("文件大小超过限制: %d > %d", fileSize, maxFileSize)
	}

	// 设置块大小
	if chunkSize == 0 {
		chunkSize = DefaultChunkSize
	}
	if chunkSize > DefaultMaxChunkSize {
		return nil, fmt.Errorf("块大小超过限制: %d > %d", chunkSize, DefaultMaxChunkSize)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[sessionID]; exists {
		return nil, fmt.Errorf("会话ID已存在: %s", sessionID)
	}

	// 创建临时文件
	tempDir := s.getTempDir()
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %v", err)
	}

	tempFilePath := filepath.Join(tempDir, fmt.Sprintf("%s_%s.tmp", uploadType, sessionID))
	tempFile, err := os.OpenFile(tempFilePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %v", err)
	}

	if err := tempFile.Truncate(fileSize); err != nil {
		err := tempFile.Close()
		if err != nil {
			return nil, err
		}
		err = os.Remove(tempFilePath)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("预分配文件空间失败: %v", err)
	}

	totalChunks := int((fileSize + chunkSize - 1) / chunkSize)
	now := time.Now()

	session := &UploadSession{
		SessionID:      sessionID,
		UploadType:     uploadType,
		FileName:       fileName,
		FileSize:       fileSize,
		ChunkSize:      chunkSize,
		TotalChunks:    totalChunks,
		UploadedChunks: make(map[int]bool),
		TempFilePath:   tempFilePath,
		Metadata:       metadata,
		Status:         "pending",
		CreatedAt:      now,
		ExpiresAt:      now.Add(DefaultSessionTimeout),
		LastActivity:   now,
	}

	s.sessions[sessionID] = &sessionWrapper{
		session:  session,
		tempFile: tempFile,
	}

	logx.Infof("初始化上传: sessionID=%s, type=%s, file=%s, size=%d, chunks=%d",
		sessionID, uploadType, fileName, fileSize, totalChunks)

	return session, nil
}

// WriteChunk 写入块（通用）
func (s *Service) WriteChunk(sessionID string, chunkIndex int, data []byte) error {
	s.mu.RLock()
	wrapper, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	wrapper.mu.Lock()
	defer wrapper.mu.Unlock()

	session := wrapper.session

	if chunkIndex < 0 || chunkIndex >= session.TotalChunks {
		return fmt.Errorf("块索引超出范围: %d", chunkIndex)
	}

	if session.UploadedChunks[chunkIndex] {
		return nil // 已上传，跳过
	}

	offset := int64(chunkIndex) * session.ChunkSize
	_, err := wrapper.tempFile.WriteAt(data, offset)
	if err != nil {
		return fmt.Errorf("写入文件块失败: %v", err)
	}

	session.UploadedChunks[chunkIndex] = true
	session.Status = "uploading"
	session.LastActivity = time.Now()

	return nil
}

// GetStatus 获取状态（通用）
func (s *Service) GetStatus(sessionID string) (*UploadSession, []int, []int, error) {
	s.mu.RLock()
	wrapper, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return nil, nil, nil, fmt.Errorf("会话不存在: %s", sessionID)
	}

	wrapper.mu.Lock()
	defer wrapper.mu.Unlock()

	session := wrapper.session

	uploadedChunks := make([]int, 0, len(session.UploadedChunks))
	for idx := range session.UploadedChunks {
		uploadedChunks = append(uploadedChunks, idx)
	}

	missingChunks := make([]int, 0)
	for i := 0; i < session.TotalChunks; i++ {
		if !session.UploadedChunks[i] {
			missingChunks = append(missingChunks, i)
		}
	}

	return session, uploadedChunks, missingChunks, nil
}

// Complete 完成上传（通用，但需要业务回调）
func (s *Service) Complete(ctx context.Context, sessionID string, callback UploadCompleteCallback) (*CompleteResult, error) {
	// 从map中获取并删除会话
	s.mu.Lock()
	wrapper, exists := s.sessions[sessionID]
	if !exists {
		s.mu.Unlock()
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}
	delete(s.sessions, sessionID)
	s.mu.Unlock()

	wrapper.mu.Lock()
	defer wrapper.mu.Unlock()

	session := wrapper.session

	// 验证所有块已上传
	if len(session.UploadedChunks) != session.TotalChunks {
		return nil, fmt.Errorf("文件块未完全上传: %d/%d", len(session.UploadedChunks), session.TotalChunks)
	}

	// 关闭临时文件
	if wrapper.tempFile != nil {
		err := wrapper.tempFile.Sync()
		if err != nil {
			return nil, err
		}
		err = wrapper.tempFile.Close()
		if err != nil {
			return nil, err
		}
		wrapper.tempFile = nil
	}

	// 确保在函数结束时删除临时文件
	defer func() {
		if session.TempFilePath != "" {
			err := os.Remove(session.TempFilePath)
			if err != nil {
				return
			}
			logx.Infof("已删除临时文件: %s", session.TempFilePath)
		}
	}()

	// 计算校验和
	checksum, err := s.calculateChecksum(session.TempFilePath)
	if err != nil {
		return nil, fmt.Errorf("计算校验和失败: %v", err)
	}

	// 调用业务回调执行实际上传
	startTime := time.Now()
	targetPath, metadata, err := callback(ctx, session, session.TempFilePath)
	if err != nil {
		return nil, fmt.Errorf("执行上传失败: %v", err)
	}

	result := &CompleteResult{
		TargetPath: targetPath,
		FileSize:   session.FileSize,
		Checksum:   checksum,
		Metadata:   metadata,
		UploadTime: time.Since(startTime).Milliseconds(),
	}

	session.Status = "completed"

	logx.Infof("完成上传: sessionID=%s, type=%s, target=%s, time=%dms",
		sessionID, session.UploadType, targetPath, result.UploadTime)

	return result, nil
}

// Cancel 取消上传（通用）
func (s *Service) Cancel(sessionID string) error {
	s.mu.Lock()
	wrapper, exists := s.sessions[sessionID]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("会话不存在: %s", sessionID)
	}
	delete(s.sessions, sessionID)
	s.mu.Unlock()

	wrapper.mu.Lock()
	defer wrapper.mu.Unlock()

	if wrapper.tempFile != nil {
		err := wrapper.tempFile.Close()
		if err != nil {
			return err
		}
	}
	if wrapper.session.TempFilePath != "" {
		err := os.Remove(wrapper.session.TempFilePath)
		if err != nil {
			return err
		}
	}

	wrapper.session.Status = "canceled"
	return nil
}

func (s *Service) calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

	hash := md5.New()
	buffer := make([]byte, 8192)

	for {
		n, err := file.Read(buffer)
		if n > 0 {
			hash.Write(buffer[:n])
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func (s *Service) startCleanupTask() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	logx.Info("上传会话清理任务已启动")

	for {
		select {
		case <-s.ctx.Done():
			logx.Info("上传会话清理任务已停止")
			return
		case <-ticker.C:
			s.cleanupExpiredSessions()
		}
	}
}

func (s *Service) cleanupExpiredSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for sessionID, wrapper := range s.sessions {
		wrapper.mu.Lock()
		if now.After(wrapper.session.ExpiresAt) || now.Sub(wrapper.session.LastActivity) > DefaultSessionTimeout {
			if wrapper.tempFile != nil {
				err := wrapper.tempFile.Close()
				if err != nil {
					return
				}
			}
			if wrapper.session.TempFilePath != "" {
				err := os.Remove(wrapper.session.TempFilePath)
				if err != nil {
					return
				}
			}
			wrapper.mu.Unlock()
			delete(s.sessions, sessionID)
			expiredCount++
			logx.Infof("清理过期会话: %s", sessionID)
		} else {
			wrapper.mu.Unlock()
		}
	}

	if expiredCount > 0 {
		logx.Infof("清理了 %d 个过期会话", expiredCount)
	}

	// 清理旧的临时目录
	s.cleanupOldTempDirs()
}

func (s *Service) cleanupOldTempDirs() {
	baseDir := filepath.Join(s.localCacheDir, "upload_temp")

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return
	}

	now := time.Now()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirDate, err := time.Parse("20060102", entry.Name())
		if err != nil {
			continue
		}

		// 删除3天前的目录
		if now.Sub(dirDate) > 3*24*time.Hour {
			dirPath := filepath.Join(baseDir, entry.Name())
			if err := os.RemoveAll(dirPath); err != nil {
				logx.Errorf("删除旧临时目录失败: %s, error: %v", dirPath, err)
			} else {
				logx.Infof("删除旧临时目录: %s", dirPath)
			}
		}
	}
}

func (s *Service) registerShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-c
		logx.Info("收到关闭信号，开始清理上传会话...")
		s.Shutdown()
		os.Exit(0)
	}()
}

func (s *Service) Shutdown() {
	logx.Info("正在关闭上传会话管理器...")

	s.cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	for sessionID, wrapper := range s.sessions {
		wrapper.mu.Lock()
		if wrapper.tempFile != nil {
			err := wrapper.tempFile.Close()
			if err != nil {
				return
			}
		}
		if wrapper.session.TempFilePath != "" {
			err := os.Remove(wrapper.session.TempFilePath)
			if err != nil {
				return
			}
		}
		wrapper.mu.Unlock()
		delete(s.sessions, sessionID)
	}

	logx.Info("上传会话管理器已关闭")
}
