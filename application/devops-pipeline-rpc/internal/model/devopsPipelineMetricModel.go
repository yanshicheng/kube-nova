package model

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsPipelineMetricCollectionName = "devops_pipeline_metric"

var ErrInvalidMetricKey = errors.New("pipeline metric dimensions incomplete")

type DevopsPipelineMetric struct {
	ID                    bson.ObjectID `bson:"_id,omitempty"`
	MetricKey             string        `bson:"metricKey,omitempty"`
	ProjectID             string        `bson:"projectId,omitempty"`
	ProjectName           string        `bson:"projectName,omitempty"`
	ProjectCode           string        `bson:"projectCode,omitempty"`
	SystemID              string        `bson:"systemId,omitempty"`
	SystemName            string        `bson:"systemName,omitempty"`
	SystemCode            string        `bson:"systemCode,omitempty"`
	EnvironmentID         string        `bson:"environmentId,omitempty"`
	EnvironmentName       string        `bson:"environmentName,omitempty"`
	EnvironmentCode       string        `bson:"environmentCode,omitempty"`
	PipelineID            string        `bson:"pipelineId,omitempty"`
	PipelineName          string        `bson:"pipelineName,omitempty"`
	PipelineCode          string        `bson:"pipelineCode,omitempty"`
	BuildChannelBindingID string        `bson:"buildChannelBindingId,omitempty"`
	ChannelID             string        `bson:"channelId,omitempty"`
	ChannelName           string        `bson:"channelName,omitempty"`
	BuildCount            int64         `bson:"buildCount,omitempty"`
	SuccessCount          int64         `bson:"successCount,omitempty"`
	FailureCount          int64         `bson:"failureCount,omitempty"`
	AbortedCount          int64         `bson:"abortedCount,omitempty"`
	TotalDurationSeconds  int64         `bson:"totalDurationSeconds,omitempty"`
	LastRunID             string        `bson:"lastRunId,omitempty"`
	LastRunStatus         string        `bson:"lastRunStatus,omitempty"`
	LastRunAt             time.Time     `bson:"lastRunAt,omitempty"`
	CreateAt              time.Time     `bson:"createAt,omitempty"`
	UpdateAt              time.Time     `bson:"updateAt,omitempty"`
	IsDeleted             bool          `bson:"isDeleted"`
}

type PipelineMetricIncrement struct {
	BuildCount           int64
	SuccessCount         int64
	FailureCount         int64
	AbortedCount         int64
	TotalDurationSeconds int64
}

type MetricFilter struct {
	ProjectID  string
	ProjectIDs []string
	Restricted bool
	Limit      int64
}

type DevopsPipelineMetricModel struct {
	conn *mon.Model
}

func NewDevopsPipelineMetricModel(url, db string) *DevopsPipelineMetricModel {
	return &DevopsPipelineMetricModel{
		conn: mon.MustNewModel(url, db, DevopsPipelineMetricCollectionName),
	}
}

func (m *DevopsPipelineMetricModel) UpsertFromRun(ctx context.Context, run *DevopsPipelineRun, inc PipelineMetricIncrement) error {
	if run == nil {
		return nil
	}
	key := pipelineMetricKey(run)
	if key == "" {
		return ErrInvalidMetricKey
	}
	t := now()
	setOnInsert := bson.M{
		"_id":                   bson.NewObjectID(),
		"metricKey":             key,
		"projectId":             run.ProjectID,
		"systemId":              run.SystemID,
		"environmentId":         run.EnvironmentID,
		"pipelineId":            run.PipelineID,
		"buildChannelBindingId": run.BuildChannelBindingID,
		"channelId":             run.ChannelID,
		"createAt":              t,
		"isDeleted":             false,
	}
	_, err := m.conn.UpdateOne(ctx,
		bson.M{"metricKey": key, "isDeleted": false},
		bson.M{
			"$setOnInsert": setOnInsert,
			"$set": bson.M{
				"projectName":     run.ProjectName,
				"projectCode":     run.ProjectCode,
				"systemName":      run.SystemName,
				"systemCode":      run.SystemCode,
				"environmentName": run.EnvironmentName,
				"environmentCode": run.EnvironmentCode,
				"pipelineName":    run.PipelineName,
				"pipelineCode":    run.PipelineCode,
				"channelName":     run.ChannelName,
				"lastRunId":       run.ID.Hex(),
				"lastRunStatus":   run.Status,
				"lastRunAt":       run.FinishedAt,
				"updateAt":        t,
			},
			"$inc": bson.M{
				"buildCount":           inc.BuildCount,
				"successCount":         inc.SuccessCount,
				"failureCount":         inc.FailureCount,
				"abortedCount":         inc.AbortedCount,
				"totalDurationSeconds": inc.TotalDurationSeconds,
			},
		},
		options.UpdateOne().SetUpsert(true),
	)
	return err
}

func (m *DevopsPipelineMetricModel) List(ctx context.Context, filter MetricFilter) ([]*DevopsPipelineMetric, error) {
	query := metricQuery(filter)
	opts := options.Find().SetSort(bson.D{{Key: "buildCount", Value: -1}, {Key: "createAt", Value: -1}})
	if filter.Limit > 0 {
		opts.SetLimit(filter.Limit)
	}
	var data []*DevopsPipelineMetric
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, err
	}
	return data, nil
}

func (m *DevopsPipelineMetricModel) Summary(ctx context.Context, filter MetricFilter) (*DevopsPipelineMetric, error) {
	var rows []DevopsPipelineMetric
	pipeline := []bson.M{
		{"$match": metricQuery(filter)},
		{"$group": bson.M{
			"_id":                  nil,
			"buildCount":           bson.M{"$sum": "$buildCount"},
			"successCount":         bson.M{"$sum": "$successCount"},
			"failureCount":         bson.M{"$sum": "$failureCount"},
			"abortedCount":         bson.M{"$sum": "$abortedCount"},
			"totalDurationSeconds": bson.M{"$sum": "$totalDurationSeconds"},
		}},
		{"$project": bson.M{
			"_id":                  0,
			"buildCount":           1,
			"successCount":         1,
			"failureCount":         1,
			"abortedCount":         1,
			"totalDurationSeconds": 1,
		}},
	}
	if err := m.conn.Aggregate(ctx, &rows, pipeline); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return &DevopsPipelineMetric{}, nil
	}
	return &rows[0], nil
}

func (m *DevopsPipelineMetricModel) GroupRank(ctx context.Context, filter MetricFilter, groupFields map[string]string, limit int64) ([]*DevopsPipelineMetric, error) {
	groupID := bson.M{}
	for alias, field := range groupFields {
		groupID[alias] = "$" + field
	}
	project := bson.M{
		"_id":                  0,
		"buildCount":           1,
		"successCount":         1,
		"failureCount":         1,
		"abortedCount":         1,
		"totalDurationSeconds": 1,
	}
	for alias := range groupFields {
		project[alias] = "$_id." + alias
	}
	pipeline := []bson.M{
		{"$match": metricQuery(filter)},
		{"$group": bson.M{
			"_id":                  groupID,
			"buildCount":           bson.M{"$sum": "$buildCount"},
			"successCount":         bson.M{"$sum": "$successCount"},
			"failureCount":         bson.M{"$sum": "$failureCount"},
			"abortedCount":         bson.M{"$sum": "$abortedCount"},
			"totalDurationSeconds": bson.M{"$sum": "$totalDurationSeconds"},
		}},
		{"$project": project},
		{"$sort": bson.M{"buildCount": -1}},
	}
	if limit > 0 {
		pipeline = append(pipeline, bson.M{"$limit": limit})
	}
	var data []*DevopsPipelineMetric
	if err := m.conn.Aggregate(ctx, &data, pipeline); err != nil {
		return nil, err
	}
	return data, nil
}

func metricQuery(filter MetricFilter) bson.M {
	query := bson.M{"isDeleted": false, "pipelineId": bson.M{"$ne": ""}}
	if filter.ProjectID != "" {
		if filter.Restricted && !stringSliceContains(filter.ProjectIDs, filter.ProjectID) {
			query["projectId"] = "__no_access__"
			return query
		}
		query["projectId"] = filter.ProjectID
	} else if filter.Restricted {
		if len(filter.ProjectIDs) == 0 {
			query["projectId"] = "__no_access__"
			return query
		}
		query["projectId"] = bson.M{"$in": filter.ProjectIDs}
	}
	return query
}

func pipelineMetricKey(run *DevopsPipelineRun) string {
	parts := []string{
		strings.TrimSpace(run.ProjectID),
		strings.TrimSpace(run.SystemID),
		strings.TrimSpace(run.EnvironmentID),
		strings.TrimSpace(run.PipelineID),
		strings.TrimSpace(run.BuildChannelBindingID),
	}
	for _, item := range parts {
		if item == "" {
			return ""
		}
	}
	return strings.Join(parts, "|")
}
