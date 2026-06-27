package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsPipelineRunArtifactCollectionName = "devops_pipeline_run_artifact"

type DevopsPipelineRunArtifact struct {
	ID           bson.ObjectID `bson:"_id,omitempty"`
	RunID        string        `bson:"runId,omitempty"`
	StageID      string        `bson:"stageId,omitempty"`
	StepID       string        `bson:"stepId,omitempty"`
	Name         string        `bson:"name,omitempty"`
	Type         string        `bson:"type,omitempty"`
	Bucket       string        `bson:"bucket,omitempty"`
	ObjectKey    string        `bson:"objectKey,omitempty"`
	Size         int64         `bson:"size,omitempty"`
	ContentType  string        `bson:"contentType,omitempty"`
	Status       string        `bson:"status,omitempty"`
	ErrorMessage string        `bson:"errorMessage,omitempty"`
	CreatedBy    string        `bson:"createdBy,omitempty"`
	UpdatedBy    string        `bson:"updatedBy,omitempty"`
	CreateAt     time.Time     `bson:"createAt,omitempty"`
	UpdateAt     time.Time     `bson:"updateAt,omitempty"`
	IsDeleted    bool          `bson:"isDeleted"`
}

type DevopsPipelineRunArtifactModel struct {
	conn *mon.Model
}

func NewDevopsPipelineRunArtifactModel(url, db string) *DevopsPipelineRunArtifactModel {
	return &DevopsPipelineRunArtifactModel{
		conn: mon.MustNewModel(url, db, DevopsPipelineRunArtifactCollectionName),
	}
}

func (m *DevopsPipelineRunArtifactModel) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "runId", Value: 1}, {Key: "isDeleted", Value: 1}, {Key: "createAt", Value: 1}},
			Options: options.Index().SetName("idx_artifact_run_created"),
		},
		{
			Keys:    bson.D{{Key: "runId", Value: 1}, {Key: "stepId", Value: 1}, {Key: "objectKey", Value: 1}, {Key: "status", Value: 1}, {Key: "isDeleted", Value: 1}},
			Options: options.Index().SetName("idx_artifact_exists"),
		},
	})
	return err
}

func (m *DevopsPipelineRunArtifactModel) Insert(ctx context.Context, data *DevopsPipelineRunArtifact) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	if data.Status == "" {
		data.Status = "success"
	}
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsPipelineRunArtifactModel) Exists(ctx context.Context, runID, stepID, objectKey, status string) bool {
	query := bson.M{"runId": runID, "isDeleted": false}
	if stepID != "" {
		query["stepId"] = stepID
	}
	if objectKey != "" {
		query["objectKey"] = objectKey
	}
	if status != "" {
		query["status"] = status
	}
	total, err := m.conn.CountDocuments(ctx, query)
	return err == nil && total > 0
}

func (m *DevopsPipelineRunArtifactModel) DeleteByRun(ctx context.Context, runID string) error {
	_, err := m.conn.UpdateMany(ctx,
		bson.M{"runId": runID, "isDeleted": false},
		bson.M{"$set": bson.M{"isDeleted": true, "updateAt": now()}},
	)
	return err
}

func (m *DevopsPipelineRunArtifactModel) DeleteByRuns(ctx context.Context, runIDs []string) error {
	if len(runIDs) == 0 {
		return nil
	}
	_, err := m.conn.UpdateMany(ctx,
		bson.M{"runId": bson.M{"$in": runIDs}, "isDeleted": false},
		bson.M{"$set": bson.M{"isDeleted": true, "updateAt": now()}},
	)
	return err
}

func (m *DevopsPipelineRunArtifactModel) ListByRun(ctx context.Context, runID string) ([]*DevopsPipelineRunArtifact, error) {
	var data []*DevopsPipelineRunArtifact
	err := m.conn.Find(ctx, &data, bson.M{"runId": runID, "isDeleted": false}, options.Find().SetSort(bson.D{{Key: "createAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	return data, nil
}
