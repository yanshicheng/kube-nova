package executionservicelogic

import (
	"testing"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
)

func TestShouldDisablePreviousJenkinsJob(t *testing.T) {
	base := &model.DevopsPipeline{
		EngineType:            engineJenkins,
		SystemID:              "system-a",
		EnvironmentID:         "dev",
		BuildChannelBindingID: "jenkins-a",
		JobFullName:           "project/job-a",
	}
	same := *base
	if shouldDisablePreviousJenkinsJob(base, &same) {
		t.Fatalf("same Jenkins target should not disable previous job")
	}

	changedJob := same
	changedJob.JobFullName = "project/job-b"
	if !shouldDisablePreviousJenkinsJob(base, &changedJob) {
		t.Fatalf("changed Jenkins job should disable previous job")
	}

	changedBinding := same
	changedBinding.BuildChannelBindingID = "jenkins-b"
	if !shouldDisablePreviousJenkinsJob(base, &changedBinding) {
		t.Fatalf("changed Jenkins binding should disable previous job")
	}

	tektonCurrent := same
	tektonCurrent.EngineType = engineTekton
	if !shouldDisablePreviousJenkinsJob(base, &tektonCurrent) {
		t.Fatalf("leaving Jenkins should disable previous job")
	}

	missingJob := *base
	missingJob.JobFullName = ""
	if shouldDisablePreviousJenkinsJob(&missingJob, &changedJob) {
		t.Fatalf("missing previous Jenkins job should not disable")
	}
}
