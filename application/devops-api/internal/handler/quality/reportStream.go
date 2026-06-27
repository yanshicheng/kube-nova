package quality

import (
	"fmt"
	"io"
	"mime"
	"net/http"

	logicquality "github.com/yanshicheng/kube-nova/application/devops-api/internal/logic/quality"
)

func writeReportObjectStream(w http.ResponseWriter, stream *logicquality.ReportObjectStream, disposition string) {
	defer stream.Reader.Close()
	contentType := stream.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": stream.FileName}))
	if stream.Size > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", stream.Size))
	}
	if disposition == "inline" {
		w.Header().Set("Content-Security-Policy", "sandbox")
		w.Header().Set("X-Content-Type-Options", "nosniff")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, stream.Reader)
}
