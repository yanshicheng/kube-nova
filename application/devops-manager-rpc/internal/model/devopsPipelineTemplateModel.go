package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsPipelineTemplateCollectionName = "devops_pipeline_template"

type PipelineTemplateStep struct {
	ID             string                   `bson:"id,omitempty" json:"id"`
	StepID         string                   `bson:"stepId,omitempty" json:"stepId"`
	StepCode       string                   `bson:"stepCode,omitempty" json:"stepCode"`
	StepName       string                   `bson:"stepName,omitempty" json:"stepName"`
	NodeName       string                   `bson:"nodeName,omitempty" json:"nodeName"`
	ContainerName  string                   `bson:"containerName,omitempty" json:"containerName"`
	StepType       string                   `bson:"stepType,omitempty" json:"stepType"`
	Icon           string                   `bson:"icon,omitempty" json:"icon"`
	IconColor      string                   `bson:"iconColor,omitempty" json:"iconColor"`
	ParentNodeID   string                   `bson:"parentNodeId,omitempty" json:"parentNodeId"`
	BranchType     string                   `bson:"branchType,omitempty" json:"branchType"`
	SortOrder      int64                    `bson:"sortOrder,omitempty" json:"sortOrder"`
	Enabled        bool                     `bson:"enabled,omitempty" json:"enabled"`
	X              int64                    `bson:"x,omitempty" json:"x"`
	Y              int64                    `bson:"y,omitempty" json:"y"`
	ParamValues    string                   `bson:"paramValues,omitempty" json:"paramValues"`
	ArtifactConfig DevopsStepArtifactConfig `bson:"artifactConfig,omitempty" json:"artifactConfig"`
}

type DevopsPipelineTemplate struct {
	ID                         bson.ObjectID          `bson:"_id,omitempty"`
	Name                       string                 `bson:"name,omitempty"`
	Code                       string                 `bson:"code,omitempty"`
	Icon                       string                 `bson:"icon,omitempty"`
	IconColor                  string                 `bson:"iconColor,omitempty"`
	Description                string                 `bson:"description,omitempty"`
	Scope                      string                 `bson:"scope,omitempty"`
	ProjectID                  string                 `bson:"projectId,omitempty"`
	EngineType                 string                 `bson:"engineType,omitempty"`
	Steps                      []PipelineTemplateStep `bson:"steps,omitempty"`
	TektonDagConfig            string                 `bson:"tektonDagConfig,omitempty"`
	TektonRunPolicy            string                 `bson:"tektonRunPolicy,omitempty"`
	TektonTriggerConfig        string                 `bson:"tektonTriggerConfig,omitempty"`
	TektonPrunerPolicyRef      string                 `bson:"tektonPrunerPolicyRef,omitempty"`
	TektonPipelineYamlSnapshot string                 `bson:"tektonPipelineYamlSnapshot,omitempty"`
	Status                     int64                  `bson:"status"`
	CreatedBy                  string                 `bson:"createdBy,omitempty"`
	UpdatedBy                  string                 `bson:"updatedBy,omitempty"`
	CreateAt                   time.Time              `bson:"createAt,omitempty"`
	UpdateAt                   time.Time              `bson:"updateAt,omitempty"`
	IsDeleted                  bool                   `bson:"isDeleted"`
}

type DevopsPipelineTemplateListFilter struct {
	Name               string
	Code               string
	Scope              string
	ProjectID          string
	ProjectIDs         []string
	RestrictProjectIDs bool
	EngineType         string
	Status             int64
	Page               uint64
	PageSize           uint64
}

type DevopsPipelineTemplateModel struct {
	conn *mon.Model
}

func NewDevopsPipelineTemplateModel(url, db string) *DevopsPipelineTemplateModel {
	return &DevopsPipelineTemplateModel{
		conn: mon.MustNewModel(url, db, DevopsPipelineTemplateCollectionName),
	}
}

func (m *DevopsPipelineTemplateModel) Insert(ctx context.Context, data *DevopsPipelineTemplate) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsPipelineTemplateModel) FindOne(ctx context.Context, id string) (*DevopsPipelineTemplate, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsPipelineTemplate
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsPipelineTemplateModel) FindOneByCode(ctx context.Context, code string) (*DevopsPipelineTemplate, error) {
	var data DevopsPipelineTemplate
	err := m.conn.FindOne(ctx, &data, bson.M{"code": code, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsPipelineTemplateModel) CountByStepID(ctx context.Context, stepID string) (uint64, error) {
	total, err := m.conn.CountDocuments(ctx, bson.M{"steps.stepId": stepID, "isDeleted": false})
	if err != nil {
		return 0, err
	}
	return uint64(total), nil
}

func (m *DevopsPipelineTemplateModel) Update(ctx context.Context, data *DevopsPipelineTemplate) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"name":                       data.Name,
			"icon":                       data.Icon,
			"iconColor":                  data.IconColor,
			"description":                data.Description,
			"scope":                      data.Scope,
			"projectId":                  data.ProjectID,
			"engineType":                 data.EngineType,
			"steps":                      data.Steps,
			"tektonDagConfig":            data.TektonDagConfig,
			"tektonRunPolicy":            data.TektonRunPolicy,
			"tektonTriggerConfig":        data.TektonTriggerConfig,
			"tektonPrunerPolicyRef":      data.TektonPrunerPolicyRef,
			"tektonPipelineYamlSnapshot": data.TektonPipelineYamlSnapshot,
			"status":                     data.Status,
			"updatedBy":                  data.UpdatedBy,
			"updateAt":                   data.UpdateAt,
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

func (m *DevopsPipelineTemplateModel) UpdateStatus(ctx context.Context, id string, status int64, updatedBy string) error {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return err
	}
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": oid, "isDeleted": false},
		bson.M{"$set": bson.M{"status": status, "updatedBy": updatedBy, "updateAt": now()}},
	)
	if err != nil {
		return err
	}
	if isNotFoundUpdate(res) {
		return ErrNotFound
	}
	return nil
}

func (m *DevopsPipelineTemplateModel) UpdateSteps(ctx context.Context, id string, steps []PipelineTemplateStep, updatedBy string) error {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return err
	}
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": oid, "isDeleted": false},
		bson.M{"$set": bson.M{"steps": steps, "updatedBy": updatedBy, "updateAt": now()}},
	)
	if err != nil {
		return err
	}
	if isNotFoundUpdate(res) {
		return ErrNotFound
	}
	return nil
}

func (m *DevopsPipelineTemplateModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsPipelineTemplateModel) List(ctx context.Context, filter DevopsPipelineTemplateListFilter) ([]*DevopsPipelineTemplate, uint64, error) {
	query := bson.M{"isDeleted": false}
	if filter.Name != "" {
		query["name"] = bson.M{"$regex": filter.Name, "$options": "i"}
	}
	if filter.Code != "" {
		query["code"] = bson.M{"$regex": filter.Code, "$options": "i"}
	}
	switch filter.Scope {
	case "system":
		query["$or"] = templateSystemScopeQuery()
	case "project":
		projectQuery := templateProjectScopeQuery(filter.ProjectIDs, filter.ProjectID, filter.RestrictProjectIDs)
		if len(projectQuery) == 0 {
			query["projectId"] = "__none__"
		} else {
			query["$or"] = projectQuery
		}
	default:
		projectQuery := templateProjectScopeQuery(filter.ProjectIDs, filter.ProjectID, filter.RestrictProjectIDs)
		systemQuery := templateSystemScopeQuery()
		if len(projectQuery) == 0 {
			query["$or"] = systemQuery
		} else {
			query["$or"] = append(systemQuery, projectQuery...)
		}
	}
	if filter.EngineType != "" {
		query["engineType"] = filter.EngineType
	}
	if filter.Status >= 0 {
		query["status"] = filter.Status
	}

	total, err := m.conn.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	page := normalizePage(filter.Page, filter.PageSize)
	opts := options.Find().
		SetSort(bson.D{{Key: "createAt", Value: -1}}).
		SetSkip(int64((page.Page - 1) * page.PageSize)).
		SetLimit(int64(page.PageSize))

	var data []*DevopsPipelineTemplate
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}

func stringInSlice(value string, items []string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func templateSystemScopeQuery() []bson.M {
	return []bson.M{
		{"scope": "system"},
		{"scope": "", "projectId": ""},
		{"scope": bson.M{"$exists": false}, "projectId": ""},
		{"scope": bson.M{"$exists": false}, "projectId": bson.M{"$exists": false}},
	}
}

func templateProjectScopeQuery(projectIDs []string, projectID string, restricted bool) []bson.M {
	if projectID != "" {
		if restricted && !stringInSlice(projectID, projectIDs) {
			return nil
		}
		return []bson.M{
			{"scope": "project", "projectId": projectID},
			{"scope": "", "projectId": projectID},
			{"scope": bson.M{"$exists": false}, "projectId": projectID},
		}
	}
	if restricted {
		return nil
	}
	return []bson.M{
		{"scope": "project", "projectId": bson.M{"$ne": ""}},
		{"scope": "", "projectId": bson.M{"$ne": ""}},
		{"scope": bson.M{"$exists": false}, "projectId": bson.M{"$ne": ""}},
	}
}
