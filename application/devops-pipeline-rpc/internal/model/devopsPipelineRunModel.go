package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsPipelineRunCollectionName = "devops_pipeline_run"

type DevopsPipelineRun struct {
	ID                            bson.ObjectID `bson:"_id,omitempty"`
	ProjectID                     string        `bson:"projectId,omitempty"`
	ProjectName                   string        `bson:"projectName,omitempty"`
	ProjectCode                   string        `bson:"projectCode,omitempty"`
	SystemID                      string        `bson:"systemId,omitempty"`
	SystemName                    string        `bson:"systemName,omitempty"`
	SystemCode                    string        `bson:"systemCode,omitempty"`
	EnvironmentID                 string        `bson:"environmentId,omitempty"`
	EnvironmentName               string        `bson:"environmentName,omitempty"`
	EnvironmentCode               string        `bson:"environmentCode,omitempty"`
	PipelineID                    string        `bson:"pipelineId,omitempty"`
	PipelineName                  string        `bson:"pipelineName,omitempty"`
	PipelineCode                  string        `bson:"pipelineCode,omitempty"`
	EngineType                    string        `bson:"engineType,omitempty"`
	TemplateID                    string        `bson:"templateId,omitempty"`
	BuildChannelBindingID         string        `bson:"buildChannelBindingId,omitempty"`
	ChannelID                     string        `bson:"channelId,omitempty"`
	ChannelName                   string        `bson:"channelName,omitempty"`
	JenkinsFolder                 string        `bson:"jenkinsFolder,omitempty"`
	JenkinsJobName                string        `bson:"jenkinsJobName,omitempty"`
	JenkinsJobFullName            string        `bson:"jenkinsJobFullName,omitempty"`
	JenkinsBuildNumber            int64         `bson:"jenkinsBuildNumber,omitempty"`
	JenkinsQueueID                string        `bson:"jenkinsQueueId,omitempty"`
	JenkinsBuildURL               string        `bson:"jenkinsBuildUrl,omitempty"`
	BuildID                       int64         `bson:"buildId,omitempty"`
	TektonNamespace               string        `bson:"tektonNamespace,omitempty"`
	TektonPipelineName            string        `bson:"tektonPipelineName,omitempty"`
	TektonPipelineRunName         string        `bson:"tektonPipelineRunName,omitempty"`
	TriggerType                   string        `bson:"triggerType,omitempty"`
	TriggerUserID                 uint64        `bson:"triggerUserId,omitempty"`
	TriggerUsername               string        `bson:"triggerUsername,omitempty"`
	Status                        string        `bson:"status,omitempty"`
	ParamsSnapshot                string        `bson:"paramsSnapshot,omitempty"`
	WorkspaceSnapshot             string        `bson:"workspaceSnapshot,omitempty"`
	EnvSnapshot                   string        `bson:"envSnapshot,omitempty"`
	PipelineSnapshot              string        `bson:"pipelineSnapshot,omitempty"`
	TektonPipelineYamlSnapshot    string        `bson:"tektonPipelineYamlSnapshot,omitempty"`
	TektonPipelineRunYamlSnapshot string        `bson:"tektonPipelineRunYamlSnapshot,omitempty"`
	StartedAt                     time.Time     `bson:"startedAt,omitempty"`
	FinishedAt                    time.Time     `bson:"finishedAt,omitempty"`
	DurationSeconds               int64         `bson:"durationSeconds,omitempty"`
	ErrorMessage                  string        `bson:"errorMessage,omitempty"`
	LogStorageType                string        `bson:"logStorageType,omitempty"`
	LogObjectKey                  string        `bson:"logObjectKey,omitempty"`
	LogSize                       int64         `bson:"logSize,omitempty"`
	LogArchivedAt                 time.Time     `bson:"logArchivedAt,omitempty"`
	StatsRecorded                 bool          `bson:"statsRecorded,omitempty"`
	CreatedBy                     string        `bson:"createdBy,omitempty"`
	UpdatedBy                     string        `bson:"updatedBy,omitempty"`
	CreateAt                      time.Time     `bson:"createAt,omitempty"`
	UpdateAt                      time.Time     `bson:"updateAt,omitempty"`
	IsDeleted                     bool          `bson:"isDeleted"`
}

type DevopsPipelineRunListFilter struct {
	ProjectID  string
	ProjectIDs []string
	Restricted bool
	PipelineID string
	Status     string
	FinalOnly  bool
	Unrecorded bool
	Page       uint64
	PageSize   uint64
	Limit      int64
}

type PipelineRunTrend struct {
	Date            string `bson:"date,omitempty"`
	BuildCount      int64  `bson:"buildCount,omitempty"`
	SuccessCount    int64  `bson:"successCount,omitempty"`
	FailureCount    int64  `bson:"failureCount,omitempty"`
	DurationSeconds int64  `bson:"durationSeconds,omitempty"`
}

type DevopsPipelineRunPruneFilter struct {
	PipelineID      string
	TektonNamespace string
	Statuses        []string
	FinalOnly       bool
}

type DevopsPipelineRunModel struct {
	conn *mon.Model
}

func NewDevopsPipelineRunModel(url, db string) *DevopsPipelineRunModel {
	return &DevopsPipelineRunModel{
		conn: mon.MustNewModel(url, db, DevopsPipelineRunCollectionName),
	}
}

func (m *DevopsPipelineRunModel) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "pipelineId", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_run_pipeline_created"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "projectId", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_run_project_created"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "status", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_run_status_created"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "engineType", Value: 1}, {Key: "tektonNamespace", Value: 1}, {Key: "tektonPipelineRunName", Value: 1}},
			Options: options.Index().SetName("idx_run_tekton_pipelinerun"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "pipelineId", Value: 1}, {Key: "triggerType", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_run_pipeline_trigger_created"),
		},
	})
	return err
}

func (m *DevopsPipelineRunModel) Insert(ctx context.Context, data *DevopsPipelineRun) error {
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

func (m *DevopsPipelineRunModel) FindOne(ctx context.Context, id string) (*DevopsPipelineRun, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsPipelineRun
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsPipelineRunModel) FindOneForInput(ctx context.Context, id string) (*DevopsPipelineRun, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsPipelineRun
	opts := options.FindOne().SetProjection(bson.M{
		"_id":                   1,
		"projectId":             1,
		"projectName":           1,
		"projectCode":           1,
		"systemId":              1,
		"systemName":            1,
		"systemCode":            1,
		"environmentId":         1,
		"environmentName":       1,
		"environmentCode":       1,
		"pipelineId":            1,
		"pipelineName":          1,
		"pipelineCode":          1,
		"buildChannelBindingId": 1,
		"channelId":             1,
		"channelName":           1,
		"jenkinsJobFullName":    1,
		"jenkinsBuildNumber":    1,
		"jenkinsQueueId":        1,
		"jenkinsBuildUrl":       1,
		"buildId":               1,
		"tektonNamespace":       1,
		"tektonPipelineName":    1,
		"tektonPipelineRunName": 1,
		"status":                1,
		"startedAt":             1,
		"finishedAt":            1,
		"durationSeconds":       1,
		"errorMessage":          1,
		"logStorageType":        1,
		"logObjectKey":          1,
		"logSize":               1,
		"logArchivedAt":         1,
		"statsRecorded":         1,
		"updatedBy":             1,
	})
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false}, opts)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsPipelineRunModel) Update(ctx context.Context, data *DevopsPipelineRun) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"jenkinsBuildNumber":    data.JenkinsBuildNumber,
			"jenkinsQueueId":        data.JenkinsQueueID,
			"jenkinsBuildUrl":       data.JenkinsBuildURL,
			"buildId":               data.BuildID,
			"tektonNamespace":       data.TektonNamespace,
			"tektonPipelineName":    data.TektonPipelineName,
			"tektonPipelineRunName": data.TektonPipelineRunName,
			"status":                data.Status,
			"finishedAt":            data.FinishedAt,
			"durationSeconds":       data.DurationSeconds,
			"errorMessage":          data.ErrorMessage,
			"logStorageType":        data.LogStorageType,
			"logObjectKey":          data.LogObjectKey,
			"logSize":               data.LogSize,
			"logArchivedAt":         data.LogArchivedAt,
			"statsRecorded":         data.StatsRecorded,
			"updatedBy":             data.UpdatedBy,
			"updateAt":              data.UpdateAt,
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

func (m *DevopsPipelineRunModel) UpdateRuntimeSnapshot(ctx context.Context, data *DevopsPipelineRun) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"projectName":                   data.ProjectName,
			"projectCode":                   data.ProjectCode,
			"systemName":                    data.SystemName,
			"systemCode":                    data.SystemCode,
			"environmentName":               data.EnvironmentName,
			"environmentCode":               data.EnvironmentCode,
			"channelId":                     data.ChannelID,
			"channelName":                   data.ChannelName,
			"jenkinsFolder":                 data.JenkinsFolder,
			"jenkinsJobName":                data.JenkinsJobName,
			"jenkinsJobFullName":            data.JenkinsJobFullName,
			"buildId":                       data.BuildID,
			"tektonNamespace":               data.TektonNamespace,
			"tektonPipelineName":            data.TektonPipelineName,
			"tektonPipelineRunName":         data.TektonPipelineRunName,
			"paramsSnapshot":                data.ParamsSnapshot,
			"envSnapshot":                   data.EnvSnapshot,
			"pipelineSnapshot":              data.PipelineSnapshot,
			"tektonPipelineYamlSnapshot":    data.TektonPipelineYamlSnapshot,
			"tektonPipelineRunYamlSnapshot": data.TektonPipelineRunYamlSnapshot,
			"updatedBy":                     data.UpdatedBy,
			"updateAt":                      data.UpdateAt,
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

func (m *DevopsPipelineRunModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return err
	}
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": oid, "isDeleted": false},
		bson.M{"$set": bson.M{"isDeleted": true, "updatedBy": updatedBy, "updateAt": now()}},
	)
	if err != nil {
		return err
	}
	if isNotFoundUpdate(res) {
		return ErrNotFound
	}
	return nil
}

func (m *DevopsPipelineRunModel) List(ctx context.Context, filter DevopsPipelineRunListFilter) ([]*DevopsPipelineRun, uint64, error) {
	query := bson.M{"isDeleted": false}
	if filter.ProjectID != "" {
		if filter.Restricted && !stringSliceContains(filter.ProjectIDs, filter.ProjectID) {
			return []*DevopsPipelineRun{}, 0, nil
		}
		query["projectId"] = filter.ProjectID
	} else if filter.Restricted {
		if len(filter.ProjectIDs) == 0 {
			return []*DevopsPipelineRun{}, 0, nil
		}
		query["projectId"] = bson.M{"$in": filter.ProjectIDs}
	}
	if filter.PipelineID != "" {
		query["pipelineId"] = filter.PipelineID
	}
	if filter.Status != "" {
		query["status"] = filter.Status
	}
	if filter.FinalOnly {
		query["status"] = bson.M{"$in": finalRunStatuses()}
	}
	if filter.Unrecorded {
		query["statsRecorded"] = bson.M{"$ne": true}
	}
	total, err := m.conn.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	page := normalizePage(filter.Page, filter.PageSize)
	limit := int64(page.PageSize)
	skip := int64((page.Page - 1) * page.PageSize)
	if filter.Limit > 0 {
		limit = filter.Limit
		skip = 0
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "createAt", Value: -1}}).
		SetSkip(skip).
		SetLimit(limit)
	var data []*DevopsPipelineRun
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}

func (m *DevopsPipelineRunModel) MarkStatsRecorded(ctx context.Context, id string) (bool, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return false, err
	}
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": oid, "isDeleted": false, "statsRecorded": bson.M{"$ne": true}},
		bson.M{"$set": bson.M{"statsRecorded": true, "updateAt": now()}},
	)
	if err != nil {
		return false, err
	}
	return res.ModifiedCount > 0, nil
}

func (m *DevopsPipelineRunModel) ResetStatsRecorded(ctx context.Context, id string) error {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return err
	}
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": oid, "isDeleted": false},
		bson.M{"$set": bson.M{"statsRecorded": false, "updateAt": now()}},
	)
	if err != nil {
		return err
	}
	if isNotFoundUpdate(res) {
		return ErrNotFound
	}
	return nil
}

func (m *DevopsPipelineRunModel) Trend(ctx context.Context, filter MetricFilter, days int) ([]PipelineRunTrend, error) {
	if days <= 0 {
		days = 14
	}
	query := metricQuery(filter)
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	current := now().In(loc)
	start := time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -days+1)
	query["createAt"] = bson.M{"$gte": start}
	pipeline := []bson.M{
		{"$match": query},
		{"$group": bson.M{
			"_id": bson.M{
				"$dateToString": bson.M{
					"format":   "%Y-%m-%d",
					"date":     "$createAt",
					"timezone": "+08:00",
				},
			},
			"buildCount":      bson.M{"$sum": 1},
			"successCount":    bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "success"}}, 1, 0}}},
			"failureCount":    bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$in": bson.A{"$status", bson.A{"failed", "unstable"}}}, 1, 0}}},
			"durationSeconds": bson.M{"$sum": "$durationSeconds"},
		}},
		{"$project": bson.M{
			"_id":             0,
			"date":            "$_id",
			"buildCount":      1,
			"successCount":    1,
			"failureCount":    1,
			"durationSeconds": 1,
		}},
		{"$sort": bson.M{"date": 1}},
	}
	var data []PipelineRunTrend
	if err := m.conn.Aggregate(ctx, &data, pipeline); err != nil {
		return nil, err
	}
	return data, nil
}

func (m *DevopsPipelineRunModel) FindLatestByPipeline(ctx context.Context, pipelineID string) (*DevopsPipelineRun, error) {
	var data DevopsPipelineRun
	opts := options.FindOne().
		SetSort(bson.D{{Key: "createAt", Value: -1}}).
		SetProjection(bson.M{
			"_id":                1,
			"pipelineId":         1,
			"jenkinsBuildNumber": 1,
			"jenkinsBuildUrl":    1,
			"status":             1,
			"startedAt":          1,
			"createAt":           1,
		})
	err := m.conn.FindOne(ctx, &data, bson.M{"pipelineId": pipelineID, "isDeleted": false}, opts)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsPipelineRunModel) FindLatestScheduledByPipeline(ctx context.Context, pipelineID string) (*DevopsPipelineRun, error) {
	var data DevopsPipelineRun
	opts := options.FindOne().SetSort(bson.D{{Key: "createAt", Value: -1}})
	err := m.conn.FindOne(ctx, &data, bson.M{
		"pipelineId":  pipelineID,
		"engineType":  "tekton",
		"triggerType": "scheduled",
		"isDeleted":   false,
	}, opts)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsPipelineRunModel) FindByTektonPipelineRun(ctx context.Context, namespace, name string) (*DevopsPipelineRun, error) {
	var data DevopsPipelineRun
	err := m.conn.FindOne(ctx, &data, bson.M{
		"isDeleted":             false,
		"engineType":            "tekton",
		"tektonNamespace":       namespace,
		"tektonPipelineRunName": name,
	})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsPipelineRunModel) LatestByPipelines(ctx context.Context, pipelineIDs []string) (map[string]*DevopsPipelineRun, error) {
	if len(pipelineIDs) == 0 {
		return map[string]*DevopsPipelineRun{}, nil
	}
	pipeline := []bson.M{
		{"$match": bson.M{"pipelineId": bson.M{"$in": pipelineIDs}, "isDeleted": false}},
		{"$sort": bson.M{"createAt": -1}},
		{"$group": bson.M{
			"_id": "$pipelineId",
			"doc": bson.M{"$first": "$$ROOT"},
		}},
		{"$replaceRoot": bson.M{"newRoot": "$doc"}},
		{"$project": bson.M{
			"_id":                1,
			"pipelineId":         1,
			"jenkinsBuildNumber": 1,
			"jenkinsBuildUrl":    1,
			"status":             1,
			"startedAt":          1,
		}},
	}
	var data []*DevopsPipelineRun
	if err := m.conn.Aggregate(ctx, &data, pipeline); err != nil {
		return nil, err
	}
	result := make(map[string]*DevopsPipelineRun, len(data))
	for _, item := range data {
		if item != nil && item.PipelineID != "" {
			result[item.PipelineID] = item
		}
	}
	return result, nil
}

func (m *DevopsPipelineRunModel) ListForPruner(ctx context.Context, filter DevopsPipelineRunPruneFilter) ([]*DevopsPipelineRun, error) {
	query := bson.M{"isDeleted": false, "engineType": "tekton"}
	if filter.PipelineID != "" {
		query["pipelineId"] = filter.PipelineID
	}
	if filter.TektonNamespace != "" {
		query["tektonNamespace"] = filter.TektonNamespace
	}
	if len(filter.Statuses) > 0 {
		query["status"] = bson.M{"$in": filter.Statuses}
	} else if filter.FinalOnly {
		query["status"] = bson.M{"$in": finalRunStatuses()}
	}
	opts := options.Find().SetSort(bson.D{{Key: "createAt", Value: -1}})
	var data []*DevopsPipelineRun
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, err
	}
	return data, nil
}

func (m *DevopsPipelineRunModel) DeleteSoftMany(ctx context.Context, ids []string, updatedBy string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	objectIDs := make([]bson.ObjectID, 0, len(ids))
	for _, id := range ids {
		oid, err := objectIDFromHex(id)
		if err != nil {
			return 0, err
		}
		objectIDs = append(objectIDs, oid)
	}
	res, err := m.conn.UpdateMany(ctx,
		bson.M{"_id": bson.M{"$in": objectIDs}, "isDeleted": false},
		bson.M{"$set": bson.M{"isDeleted": true, "updatedBy": updatedBy, "updateAt": now()}},
	)
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

func finalRunStatuses() bson.A {
	return bson.A{"success", "failed", "aborted", "unstable", "skipped", "finished"}
}
