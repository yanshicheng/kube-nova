package minio

import (
	"fmt"
	"io"
	"log"
)

// ProgressReader 是一个装饰器，它包装了一个 io.Reader，并在读取数据时报告进度
type ProgressReader struct {
	reader   io.Reader // 实际的数据源
	total    int64     // 总数据量，用于计算进度
	readSize int64     // 已经读取的数据量
}

// Read 实现了 io.Reader 接口
func (p *ProgressReader) Read(b []byte) (int, error) {
	n, err := p.reader.Read(b)
	p.readSize += int64(n)
	if err == nil {
		percentage := float64(p.readSize) / float64(p.total) * 100
		log.Printf("Uploaded %d out of %d bytes (%.2f%%)\n", p.readSize, p.total, percentage)
	}
	return n, err
}

// NewProgressReader 创建一个新的 ProgressReader
func NewProgressReader(reader io.Reader, total int64) *ProgressReader {
	return &ProgressReader{
		reader: reader,
		total:  total,
	}
}

// ProgressListener 用于显示上传进度
type ProgressListener struct {
	TotalSize   int64
	Uploaded    int64
	LastPrinted int64
}

func (p *ProgressListener) Write(data []byte) (int, error) {
	n := len(data)
	p.Uploaded += int64(n)
	if p.Uploaded-p.LastPrinted >= p.TotalSize/100 {
		fmt.Printf("Uploaded %d/%d bytes (%.2f%%)\n", p.Uploaded, p.TotalSize, float64(p.Uploaded)/float64(p.TotalSize)*100)
		p.LastPrinted = p.Uploaded
	}
	return n, nil
}
