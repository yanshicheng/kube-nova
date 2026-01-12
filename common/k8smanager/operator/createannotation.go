package operator

import (
	"github.com/yanshicheng/kube-nova/common/utils"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// annotation 添加注解

func injectCommonAnnotations(obj v1.Object) {
	if obj == nil {
		return
	}

	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[utils.AnnotationManagedBy] = utils.AnnotationManagedBy
	obj.SetAnnotations(annotations)
}
