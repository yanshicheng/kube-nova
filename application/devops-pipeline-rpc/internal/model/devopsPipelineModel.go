package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsPipelineCollectionName = "devops_pipeline"

type StepParamOption struct {
	Label string `bson:"label,omitempty" json:"label"`
	Value string `bson:"value,omitempty" json:"value"`
}

type CompoundParamField struct {
	Name         string            `bson:"name,omitempty" json:"name"`
	Code         string            `bson:"code,omitempty" json:"code"`
	DefaultValue string            `bson:"defaultValue,omitempty" json:"defaultValue"`
	ParamType    string            `bson:"paramType,omitempty" json:"paramType"`
	Mode         string            `bson:"mode,omitempty" json:"mode"`
	Required     bool              `bson:"required,omitempty" json:"required"`
	Readonly     bool              `bson:"readonly,omitempty" json:"readonly"`
	Description  string            `bson:"description,omitempty" json:"description"`
	SelectList   []StepParamOption `bson:"selectList,omitempty" json:"selectList"`
	AllowCustom  bool              `bson:"allowCustom,omitempty" json:"allowCustom"`
	RuntimeMode  string            `bson:"runtimeMode,omitempty" json:"runtimeMode"`
	Config       StepParamConfig   `bson:"config,omitempty" json:"config"`
}

type StepParamConfig struct {
	Options                   []StepParamOption     `bson:"options,omitempty" json:"options"`
	ChannelGroupCode          string                `bson:"channelGroupCode,omitempty" json:"channelGroupCode"`
	ChannelTypeFilter         string                `bson:"channelTypeFilter,omitempty" json:"channelTypeFilter"`
	ChannelParamCode          string                `bson:"channelParamCode,omitempty" json:"channelParamCode"`
	ChannelBindingID          string                `bson:"channelBindingId,omitempty" json:"channelBindingId"`
	MappingField              string                `bson:"mappingField,omitempty" json:"mappingField"`
	VoucherModel              string                `bson:"voucherModel,omitempty" json:"voucherModel"`
	CredentialID              string                `bson:"credentialId,omitempty" json:"credentialId"`
	CredentialMode            string                `bson:"credentialMode,omitempty" json:"credentialMode"`
	CredentialSourceParamCode string                `bson:"credentialSourceParamCode,omitempty" json:"credentialSourceParamCode"`
	ProjectParamCode          string                `bson:"projectParamCode,omitempty" json:"projectParamCode"`
	ComponentParamCode        string                `bson:"componentParamCode,omitempty" json:"componentParamCode"`
	Provider                  string                `bson:"provider,omitempty" json:"provider"`
	ValidateRemote            bool                  `bson:"validateRemote,omitempty" json:"validateRemote"`
	CompoundFields            []CompoundParamField  `bson:"compoundFields,omitempty" json:"compoundFields"`
	FullRow                   bool                  `bson:"fullRow,omitempty" json:"fullRow"`
	RenderMode                string                `bson:"renderMode,omitempty" json:"renderMode"`
	ConfigTypeID              string                `bson:"configTypeId,omitempty" json:"configTypeId"`
	ConfigTypeCode            string                `bson:"configTypeCode,omitempty" json:"configTypeCode"`
	ValueMode                 string                `bson:"valueMode,omitempty" json:"valueMode"`
	DependencyParamCodes      []StepParamDependency `bson:"dependencyParamCodes,omitempty" json:"dependencyParamCodes"`
}

type StepParamDependency struct {
	Field     string `bson:"field,omitempty" json:"field"`
	ParamCode string `bson:"paramCode,omitempty" json:"paramCode"`
}

type StepArtifactConfig struct {
	Enabled     bool   `bson:"enabled,omitempty" json:"enabled"`
	Type        string `bson:"type,omitempty" json:"type"`
	Name        string `bson:"name,omitempty" json:"name"`
	Path        string `bson:"path,omitempty" json:"path"`
	Required    bool   `bson:"required,omitempty" json:"required"`
	ContentType string `bson:"contentType,omitempty" json:"contentType"`
}

type PipelineParam struct {
	Name          string            `bson:"name,omitempty" json:"name"`
	Code          string            `bson:"code,omitempty" json:"code"`
	SourceCode    string            `bson:"sourceCode,omitempty" json:"sourceCode"`
	StepNodeID    string            `bson:"stepNodeId,omitempty" json:"stepNodeId"`
	DefaultValue  string            `bson:"defaultValue,omitempty" json:"defaultValue"`
	CurrentValue  string            `bson:"currentValue,omitempty" json:"currentValue"`
	ParamType     string            `bson:"paramType,omitempty" json:"paramType"`
	Mode          string            `bson:"mode,omitempty" json:"mode"`
	Required      bool              `bson:"required,omitempty" json:"required"`
	Readonly      bool              `bson:"readonly,omitempty" json:"readonly"`
	Description   string            `bson:"description,omitempty" json:"description"`
	SelectList    []StepParamOption `bson:"selectList,omitempty" json:"selectList"`
	AllowCustom   bool              `bson:"allowCustom,omitempty" json:"allowCustom"`
	SortOrder     int64             `bson:"sortOrder,omitempty" json:"sortOrder"`
	RuntimeMode   string            `bson:"runtimeMode,omitempty" json:"runtimeMode"`
	RuntimeConfig bool              `bson:"runtimeConfig,omitempty" json:"runtimeConfig"`
	Config        StepParamConfig   `bson:"config,omitempty" json:"config"`
}

type PipelineStep struct {
	ID             string             `bson:"id,omitempty" json:"id"`
	StepID         string             `bson:"stepId,omitempty" json:"stepId"`
	StepCode       string             `bson:"stepCode,omitempty" json:"stepCode"`
	StepName       string             `bson:"stepName,omitempty" json:"stepName"`
	NodeName       string             `bson:"nodeName,omitempty" json:"nodeName"`
	ContainerName  string             `bson:"containerName,omitempty" json:"containerName"`
	StepType       string             `bson:"stepType,omitempty" json:"stepType"`
	Icon           string             `bson:"icon,omitempty" json:"icon"`
	IconColor      string             `bson:"iconColor,omitempty" json:"iconColor"`
	ParentNodeID   string             `bson:"parentNodeId,omitempty" json:"parentNodeId"`
	BranchType     string             `bson:"branchType,omitempty" json:"branchType"`
	SortOrder      int64              `bson:"sortOrder,omitempty" json:"sortOrder"`
	Enabled        bool               `bson:"enabled,omitempty" json:"enabled"`
	X              int64              `bson:"x,omitempty" json:"x"`
	Y              int64              `bson:"y,omitempty" json:"y"`
	ParamValues    string             `bson:"paramValues,omitempty" json:"paramValues"`
	StageContent   string             `bson:"stageContent,omitempty" json:"stageContent"`
	ArtifactConfig StepArtifactConfig `bson:"artifactConfig,omitempty" json:"artifactConfig"`
}

type JenkinsAgent struct {
	Type        string                  `bson:"type,omitempty" json:"type"`
	Label       string                  `bson:"label,omitempty" json:"label"`
	DockerImage string                  `bson:"dockerImage,omitempty" json:"dockerImage"`
	DockerArgs  string                  `bson:"dockerArgs,omitempty" json:"dockerArgs"`
	Raw         string                  `bson:"raw,omitempty" json:"raw"`
	ID          string                  `bson:"id,omitempty" json:"id"`
	Name        string                  `bson:"name,omitempty" json:"name"`
	AgentType   string                  `bson:"agentType,omitempty" json:"agentType"`
	MatchMode   string                  `bson:"matchMode,omitempty" json:"matchMode"`
	MatchValue  string                  `bson:"matchValue,omitempty" json:"matchValue"`
	Cloud       string                  `bson:"cloud,omitempty" json:"cloud"`
	PodYaml     string                  `bson:"podYaml,omitempty" json:"podYaml"`
	Containers  []JenkinsAgentContainer `bson:"containers,omitempty" json:"containers"`
}

type JenkinsAgentContainer struct {
	Name  string `bson:"name,omitempty" json:"name"`
	Image string `bson:"image,omitempty" json:"image"`
}

type JenkinsTool struct {
	Name    string `bson:"name,omitempty" json:"name"`
	Type    string `bson:"type,omitempty" json:"type"`
	Version string `bson:"version,omitempty" json:"version"`
}

type JenkinsOption struct {
	Name  string `bson:"name,omitempty" json:"name"`
	Value string `bson:"value,omitempty" json:"value"`
}

type JenkinsTrigger struct {
	Type    string `bson:"type,omitempty" json:"type"`
	Spec    string `bson:"spec,omitempty" json:"spec"`
	Enabled bool   `bson:"enabled,omitempty" json:"enabled"`
}

type DevopsPipeline struct {
	ID                         bson.ObjectID    `bson:"_id,omitempty"`
	ProjectID                  string           `bson:"projectId,omitempty"`
	ProjectName                string           `bson:"projectName,omitempty"`
	ProjectCode                string           `bson:"projectCode,omitempty"`
	SystemID                   string           `bson:"systemId,omitempty"`
	SystemName                 string           `bson:"systemName,omitempty"`
	SystemCode                 string           `bson:"systemCode,omitempty"`
	EnvironmentID              string           `bson:"environmentId,omitempty"`
	EnvironmentName            string           `bson:"environmentName,omitempty"`
	EnvironmentCode            string           `bson:"environmentCode,omitempty"`
	Name                       string           `bson:"name,omitempty"`
	Code                       string           `bson:"code,omitempty"`
	EngineType                 string           `bson:"engineType,omitempty"`
	TemplateID                 string           `bson:"templateId,omitempty"`
	BuildChannelBindingID      string           `bson:"buildChannelBindingId,omitempty"`
	BuildChannelName           string           `bson:"buildChannelName,omitempty"`
	Steps                      []PipelineStep   `bson:"steps,omitempty"`
	Params                     []PipelineParam  `bson:"params,omitempty"`
	Agent                      JenkinsAgent     `bson:"agent,omitempty"`
	Tools                      []JenkinsTool    `bson:"tools,omitempty"`
	Options                    []JenkinsOption  `bson:"options,omitempty"`
	Triggers                   []JenkinsTrigger `bson:"triggers,omitempty"`
	JobName                    string           `bson:"jobName,omitempty"`
	JobFullName                string           `bson:"jobFullName,omitempty"`
	TektonNamespace            string           `bson:"tektonNamespace,omitempty"`
	TektonPipelineName         string           `bson:"tektonPipelineName,omitempty"`
	TektonTemplateID           string           `bson:"tektonTemplateId,omitempty"`
	TektonTemplateSnapshot     string           `bson:"tektonTemplateSnapshot,omitempty"`
	TektonParamBindings        string           `bson:"tektonParamBindings,omitempty"`
	TektonWorkspaceBindings    string           `bson:"tektonWorkspaceBindings,omitempty"`
	TektonResourceBindings     string           `bson:"tektonResourceBindings,omitempty"`
	TektonDagConfig            string           `bson:"tektonDagConfig,omitempty"`
	TektonRunPolicy            string           `bson:"tektonRunPolicy,omitempty"`
	TektonTriggerConfig        string           `bson:"tektonTriggerConfig,omitempty"`
	TektonPrunerPolicyRef      string           `bson:"tektonPrunerPolicyRef,omitempty"`
	TektonPipelineYamlSnapshot string           `bson:"tektonPipelineYamlSnapshot,omitempty" json:"tektonPipelineYamlSnapshot"`
	TriggerMode                string           `bson:"triggerMode,omitempty"`
	ScanEnabled                bool             `bson:"scanEnabled,omitempty"`
	ScanMode                   string           `bson:"scanMode,omitempty"`
	RejectUnexpectedReports    bool             `bson:"rejectUnexpectedReports,omitempty"`
	Enforce                    bool             `bson:"enforce,omitempty"`
	ScanItems                  []ScanPlanItem   `bson:"scanItems,omitempty"`
	SyncStatus                 string           `bson:"syncStatus,omitempty"`
	SyncMessage                string           `bson:"syncMessage,omitempty"`
	LastRunStatus              string           `bson:"lastRunStatus,omitempty"`
	Status                     int64            `bson:"status"`
	CreatedBy                  string           `bson:"createdBy,omitempty"`
	UpdatedBy                  string           `bson:"updatedBy,omitempty"`
	CreateAt                   time.Time        `bson:"createAt,omitempty"`
	UpdateAt                   time.Time        `bson:"updateAt,omitempty"`
	IsDeleted                  bool             `bson:"isDeleted"`
}

type ScanPlanItem struct {
	StepID          string `bson:"stepId,omitempty"`
	StageID         string `bson:"stageId,omitempty"`
	Tool            string `bson:"tool,omitempty"`
	ToolMode        string `bson:"toolMode,omitempty"`
	ToolBindingID   string `bson:"toolBindingId,omitempty"`
	TargetType      string `bson:"targetType,omitempty"`
	TargetName      string `bson:"targetName,omitempty"`
	TargetParamCode string `bson:"targetParamCode,omitempty"`
	ReportSource    string `bson:"reportSource,omitempty"`
	ReportPath      string `bson:"reportPath,omitempty"`
	ReportFormat    string `bson:"reportFormat,omitempty"`
	Required        bool   `bson:"required,omitempty"`
	Enforce         bool   `bson:"enforce,omitempty"`
	Parser          string `bson:"parser,omitempty"`
}

type DevopsPipelineListFilter struct {
	ProjectID     string
	ProjectIDs    []string
	Restricted    bool
	SystemID      string
	EnvironmentID string
	Name          string
	Code          string
	EngineType    string
	SyncStatus    string
	LastRunStatus string
	TriggerMode   string
	Status        int64
	Page          uint64
	PageSize      uint64
}

type DevopsPipelineModel struct {
	conn *mon.Model
}

func NewDevopsPipelineModel(url, db string) *DevopsPipelineModel {
	return &DevopsPipelineModel{
		conn: mon.MustNewModel(url, db, DevopsPipelineCollectionName),
	}
}

func (m *DevopsPipelineModel) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_pipeline_deleted_created"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "projectId", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_pipeline_project_created"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "projectId", Value: 1}, {Key: "systemId", Value: 1}, {Key: "environmentId", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_pipeline_project_system_env_created"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "projectId", Value: 1}, {Key: "systemId", Value: 1}, {Key: "environmentId", Value: 1}, {Key: "code", Value: 1}},
			Options: options.Index().SetName("idx_pipeline_unique_lookup"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "lastRunStatus", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_pipeline_last_run_created"),
		},
	})
	return err
}

func (m *DevopsPipelineModel) Insert(ctx context.Context, data *DevopsPipeline) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	if data.SyncStatus == "" {
		data.SyncStatus = "pending"
	}
	if data.LastRunStatus == "" {
		data.LastRunStatus = "never"
	}
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsPipelineModel) FindOne(ctx context.Context, id string) (*DevopsPipeline, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsPipeline
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsPipelineModel) FindOneByUnique(ctx context.Context, projectID, systemID, code, environmentID string) (*DevopsPipeline, error) {
	var data DevopsPipeline
	err := m.conn.FindOne(ctx, &data, bson.M{
		"projectId":     projectID,
		"systemId":      systemID,
		"code":          code,
		"environmentId": environmentID,
		"isDeleted":     false,
	})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsPipelineModel) ListTektonScheduled(ctx context.Context, limit int64) ([]*DevopsPipeline, error) {
	if limit <= 0 {
		limit = 200
	}
	query := bson.M{
		"isDeleted":  false,
		"engineType": "tekton",
		"syncStatus": "synced",
		"status":     int64(1),
		"$or": bson.A{
			bson.M{"tektonTriggerConfig": bson.M{"$regex": "scheduleConfig"}},
			bson.M{"tektonDagConfig": bson.M{"$regex": "scheduleConfig"}},
		},
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "createAt", Value: -1}}).
		SetLimit(limit)
	var data []*DevopsPipeline
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, err
	}
	return data, nil
}

func (m *DevopsPipelineModel) Update(ctx context.Context, data *DevopsPipeline) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"systemId":                   data.SystemID,
			"systemName":                 data.SystemName,
			"systemCode":                 data.SystemCode,
			"environmentId":              data.EnvironmentID,
			"environmentName":            data.EnvironmentName,
			"environmentCode":            data.EnvironmentCode,
			"name":                       data.Name,
			"engineType":                 data.EngineType,
			"templateId":                 data.TemplateID,
			"buildChannelBindingId":      data.BuildChannelBindingID,
			"buildChannelName":           data.BuildChannelName,
			"steps":                      data.Steps,
			"params":                     data.Params,
			"agent":                      data.Agent,
			"tools":                      data.Tools,
			"options":                    data.Options,
			"triggers":                   data.Triggers,
			"jobName":                    data.JobName,
			"jobFullName":                data.JobFullName,
			"tektonNamespace":            data.TektonNamespace,
			"tektonPipelineName":         data.TektonPipelineName,
			"tektonTemplateId":           data.TektonTemplateID,
			"tektonTemplateSnapshot":     data.TektonTemplateSnapshot,
			"tektonParamBindings":        data.TektonParamBindings,
			"tektonWorkspaceBindings":    data.TektonWorkspaceBindings,
			"tektonResourceBindings":     data.TektonResourceBindings,
			"tektonDagConfig":            data.TektonDagConfig,
			"tektonRunPolicy":            data.TektonRunPolicy,
			"tektonTriggerConfig":        data.TektonTriggerConfig,
			"tektonPrunerPolicyRef":      data.TektonPrunerPolicyRef,
			"tektonPipelineYamlSnapshot": data.TektonPipelineYamlSnapshot,
			"triggerMode":                data.TriggerMode,
			"scanEnabled":                data.ScanEnabled,
			"scanMode":                   data.ScanMode,
			"rejectUnexpectedReports":    data.RejectUnexpectedReports,
			"enforce":                    data.Enforce,
			"scanItems":                  data.ScanItems,
			"syncStatus":                 data.SyncStatus,
			"syncMessage":                data.SyncMessage,
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

func (m *DevopsPipelineModel) UpdateRunStatus(ctx context.Context, id, status, updatedBy string) error {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return err
	}
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": oid, "isDeleted": false},
		bson.M{"$set": bson.M{"lastRunStatus": status, "updatedBy": updatedBy, "updateAt": now()}},
	)
	if err != nil {
		return err
	}
	if isNotFoundUpdate(res) {
		return ErrNotFound
	}
	return nil
}

func (m *DevopsPipelineModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsPipelineModel) List(ctx context.Context, filter DevopsPipelineListFilter) ([]*DevopsPipeline, uint64, error) {
	query := bson.M{"isDeleted": false}
	if filter.ProjectID != "" {
		if filter.Restricted && !stringSliceContains(filter.ProjectIDs, filter.ProjectID) {
			return []*DevopsPipeline{}, 0, nil
		}
		query["projectId"] = filter.ProjectID
	} else if filter.Restricted {
		if len(filter.ProjectIDs) == 0 {
			return []*DevopsPipeline{}, 0, nil
		}
		query["projectId"] = bson.M{"$in": filter.ProjectIDs}
	}
	if filter.SystemID != "" {
		query["systemId"] = filter.SystemID
	}
	if filter.EnvironmentID != "" {
		query["environmentId"] = filter.EnvironmentID
	}
	if filter.Name != "" {
		query["name"] = bson.M{"$regex": filter.Name, "$options": "i"}
	}
	if filter.Code != "" {
		query["code"] = bson.M{"$regex": filter.Code, "$options": "i"}
	}
	if filter.EngineType != "" {
		query["engineType"] = filter.EngineType
	}
	if filter.SyncStatus != "" {
		query["syncStatus"] = filter.SyncStatus
	}
	if filter.LastRunStatus != "" {
		query["lastRunStatus"] = filter.LastRunStatus
	}
	if filter.TriggerMode != "" {
		query["$or"] = triggerModeQuery(filter.TriggerMode)
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
		SetLimit(int64(page.PageSize)).
		SetProjection(bson.M{
			"steps":                      0,
			"params":                     0,
			"tools":                      0,
			"options":                    0,
			"agent":                      0,
			"tektonPipelineYamlSnapshot": 0,
			"isDeleted":                  0,
		})
	var data []*DevopsPipeline
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}

func triggerModeQuery(mode string) []bson.M {
	legacyTriggerModeEmpty := bson.M{
		"$or": []bson.M{
			{"triggerMode": bson.M{"$exists": false}},
			{"triggerMode": ""},
		},
	}
	scheduledTrigger := bson.M{
		"triggers": bson.M{
			"$elemMatch": bson.M{
				"enabled": true,
				"spec":    bson.M{"$ne": ""},
			},
		},
	}
	if mode == "scheduled" {
		return []bson.M{
			{"triggerMode": "scheduled"},
			{"$and": []bson.M{legacyTriggerModeEmpty, scheduledTrigger}},
		}
	}
	return []bson.M{
		{"triggerMode": "manual"},
		{"$and": []bson.M{
			legacyTriggerModeEmpty,
			{"triggers": bson.M{"$not": scheduledTrigger["triggers"]}},
		}},
	}
}
