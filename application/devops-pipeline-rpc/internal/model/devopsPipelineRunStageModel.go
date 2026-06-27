package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsPipelineRunStageCollectionName = "devops_pipeline_run_stage"

type DevopsPipelineRunStage struct {
	ID                bson.ObjectID           `bson:"_id,omitempty"`
	RunID             string                  `bson:"runId,omitempty"`
	PipelineID        string                  `bson:"pipelineId,omitempty"`
	NodeID            string                  `bson:"nodeId,omitempty"`
	StageName         string                  `bson:"stageName,omitempty"`
	StageType         string                  `bson:"stageType,omitempty"`
	Status            string                  `bson:"status,omitempty"`
	StartedAt         time.Time               `bson:"startedAt,omitempty"`
	FinishedAt        time.Time               `bson:"finishedAt,omitempty"`
	DurationSeconds   int64                   `bson:"durationSeconds,omitempty"`
	JenkinsNodeID     string                  `bson:"jenkinsNodeId,omitempty"`
	TektonTaskRunName string                  `bson:"tektonTaskRunName,omitempty"`
	TektonPodName     string                  `bson:"tektonPodName,omitempty"`
	ContainerName     string                  `bson:"containerName,omitempty"`
	ContainerNames    []string                `bson:"containerNames,omitempty"`
	TektonTaskRuns    []TektonTaskRunSnapshot `bson:"tektonTaskRuns,omitempty"`
	LogObjectKey      string                  `bson:"logObjectKey,omitempty"`
	ErrorMessage      string                  `bson:"errorMessage,omitempty"`
	CreateAt          time.Time               `bson:"createAt,omitempty"`
	UpdateAt          time.Time               `bson:"updateAt,omitempty"`
	IsDeleted         bool                    `bson:"isDeleted"`
}

type TektonTaskRunSnapshot struct {
	Name             string    `bson:"name,omitempty"`
	PipelineTaskName string    `bson:"pipelineTaskName,omitempty"`
	PodName          string    `bson:"podName,omitempty"`
	ContainerName    string    `bson:"containerName,omitempty"`
	ContainerNames   []string  `bson:"containerNames,omitempty"`
	Status           string    `bson:"status,omitempty"`
	StartedAt        time.Time `bson:"startedAt,omitempty"`
	FinishedAt       time.Time `bson:"finishedAt,omitempty"`
	DurationSeconds  int64     `bson:"durationSeconds,omitempty"`
	ErrorMessage     string    `bson:"errorMessage,omitempty"`
}

type DevopsPipelineRunStageModel struct {
	conn *mon.Model
}

func NewDevopsPipelineRunStageModel(url, db string) *DevopsPipelineRunStageModel {
	return &DevopsPipelineRunStageModel{
		conn: mon.MustNewModel(url, db, DevopsPipelineRunStageCollectionName),
	}
}

func (m *DevopsPipelineRunStageModel) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "runId", Value: 1}, {Key: "isDeleted", Value: 1}, {Key: "createAt", Value: 1}},
			Options: options.Index().SetName("idx_stage_run_created"),
		},
	})
	return err
}

func (m *DevopsPipelineRunStageModel) Insert(ctx context.Context, data *DevopsPipelineRunStage) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	if data.Status == "" {
		data.Status = "queued"
	}
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsPipelineRunStageModel) InsertMany(ctx context.Context, items []*DevopsPipelineRunStage) error {
	if len(items) == 0 {
		return nil
	}
	t := now()
	docs := make([]any, 0, len(items))
	for _, data := range items {
		if data == nil {
			continue
		}
		data.ID = bson.NewObjectID()
		data.CreateAt = t
		data.UpdateAt = t
		data.IsDeleted = false
		if data.Status == "" {
			data.Status = "queued"
		}
		docs = append(docs, data)
	}
	if len(docs) == 0 {
		return nil
	}
	_, err := m.conn.InsertMany(ctx, docs)
	return err
}

func (m *DevopsPipelineRunStageModel) DeleteByRun(ctx context.Context, runID string) error {
	_, err := m.conn.UpdateMany(ctx,
		bson.M{"runId": runID, "isDeleted": false},
		bson.M{"$set": bson.M{"isDeleted": true, "updateAt": now()}},
	)
	return err
}

func (m *DevopsPipelineRunStageModel) DeleteByRuns(ctx context.Context, runIDs []string) error {
	if len(runIDs) == 0 {
		return nil
	}
	_, err := m.conn.UpdateMany(ctx,
		bson.M{"runId": bson.M{"$in": runIDs}, "isDeleted": false},
		bson.M{"$set": bson.M{"isDeleted": true, "updateAt": now()}},
	)
	return err
}

func (m *DevopsPipelineRunStageModel) Update(ctx context.Context, data *DevopsPipelineRunStage) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"status":            data.Status,
			"startedAt":         data.StartedAt,
			"finishedAt":        data.FinishedAt,
			"durationSeconds":   data.DurationSeconds,
			"jenkinsNodeId":     data.JenkinsNodeID,
			"tektonTaskRunName": data.TektonTaskRunName,
			"tektonPodName":     data.TektonPodName,
			"containerName":     data.ContainerName,
			"containerNames":    data.ContainerNames,
			"tektonTaskRuns":    data.TektonTaskRuns,
			"logObjectKey":      data.LogObjectKey,
			"errorMessage":      data.ErrorMessage,
			"updateAt":          data.UpdateAt,
		}},
	)
	if err != nil {
		return err
	}
	if isNotFoundUpdate(res) {
		return ErrNotFound
	}
	return nil
}

func (m *DevopsPipelineRunStageModel) ListByRun(ctx context.Context, runID string) ([]*DevopsPipelineRunStage, error) {
	opts := options.Find().SetSort(bson.D{{Key: "createAt", Value: 1}})
	var data []*DevopsPipelineRunStage
	if err := m.conn.Find(ctx, &data, bson.M{"runId": runID, "isDeleted": false}, opts); err != nil {
		return nil, err
	}
	return data, nil
}

func (m *DevopsPipelineRunStageModel) FindOne(ctx context.Context, id string) (*DevopsPipelineRunStage, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsPipelineRunStage
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}
