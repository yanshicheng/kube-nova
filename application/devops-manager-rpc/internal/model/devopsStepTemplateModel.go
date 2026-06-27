package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsStepTemplateCollectionName = "devops_step_template"

type DevopsStepParamOption struct {
	Label string `bson:"label,omitempty" json:"label"`
	Value string `bson:"value,omitempty" json:"value"`
}

type DevopsCompoundParamField struct {
	Name         string                  `bson:"name,omitempty" json:"name"`
	Code         string                  `bson:"code,omitempty" json:"code"`
	DefaultValue string                  `bson:"defaultValue,omitempty" json:"defaultValue"`
	ParamType    string                  `bson:"paramType,omitempty" json:"paramType"`
	Mode         string                  `bson:"mode,omitempty" json:"mode"`
	Required     bool                    `bson:"required,omitempty" json:"required"`
	Readonly     bool                    `bson:"readonly,omitempty" json:"readonly"`
	Description  string                  `bson:"description,omitempty" json:"description"`
	SelectList   []DevopsStepParamOption `bson:"selectList,omitempty" json:"selectList"`
	AllowCustom  bool                    `bson:"allowCustom,omitempty" json:"allowCustom"`
	RuntimeMode  string                  `bson:"runtimeMode,omitempty" json:"runtimeMode"`
	Config       DevopsStepParamConfig   `bson:"config,omitempty" json:"config"`
}

type DevopsStepParamConfig struct {
	Options                   []DevopsStepParamOption     `bson:"options,omitempty" json:"options"`
	ChannelGroupCode          string                      `bson:"channelGroupCode,omitempty" json:"channelGroupCode"`
	ChannelTypeFilter         string                      `bson:"channelTypeFilter,omitempty" json:"channelTypeFilter"`
	ChannelParamCode          string                      `bson:"channelParamCode,omitempty" json:"channelParamCode"`
	ChannelBindingID          string                      `bson:"channelBindingId,omitempty" json:"channelBindingId"`
	MappingField              string                      `bson:"mappingField,omitempty" json:"mappingField"`
	VoucherModel              string                      `bson:"voucherModel,omitempty" json:"voucherModel"`
	CredentialID              string                      `bson:"credentialId,omitempty" json:"credentialId"`
	CredentialMode            string                      `bson:"credentialMode,omitempty" json:"credentialMode"`
	CredentialSourceParamCode string                      `bson:"credentialSourceParamCode,omitempty" json:"credentialSourceParamCode"`
	ProjectParamCode          string                      `bson:"projectParamCode,omitempty" json:"projectParamCode"`
	ComponentParamCode        string                      `bson:"componentParamCode,omitempty" json:"componentParamCode"`
	Provider                  string                      `bson:"provider,omitempty" json:"provider"`
	ValidateRemote            bool                        `bson:"validateRemote,omitempty" json:"validateRemote"`
	CompoundFields            []DevopsCompoundParamField  `bson:"compoundFields,omitempty" json:"compoundFields"`
	FullRow                   bool                        `bson:"fullRow,omitempty" json:"fullRow"`
	RenderMode                string                      `bson:"renderMode,omitempty" json:"renderMode"`
	ConfigTypeID              string                      `bson:"configTypeId,omitempty" json:"configTypeId"`
	ConfigTypeCode            string                      `bson:"configTypeCode,omitempty" json:"configTypeCode"`
	ValueMode                 string                      `bson:"valueMode,omitempty" json:"valueMode"`
	DependencyParamCodes      []DevopsStepParamDependency `bson:"dependencyParamCodes,omitempty" json:"dependencyParamCodes"`
}

type DevopsStepParamDependency struct {
	Field     string `bson:"field,omitempty" json:"field"`
	ParamCode string `bson:"paramCode,omitempty" json:"paramCode"`
}

type DevopsStepArtifactConfig struct {
	Enabled     bool   `bson:"enabled,omitempty" json:"enabled"`
	Type        string `bson:"type,omitempty" json:"type"`
	Name        string `bson:"name,omitempty" json:"name"`
	Path        string `bson:"path,omitempty" json:"path"`
	Required    bool   `bson:"required,omitempty" json:"required"`
	ContentType string `bson:"contentType,omitempty" json:"contentType"`
}

type DevopsTektonPropertySpec struct {
	Type string `bson:"type,omitempty" json:"type"`
}

type DevopsTektonTaskParam struct {
	Name         string                              `bson:"name,omitempty" json:"name"`
	Type         string                              `bson:"type,omitempty" json:"type"`
	DefaultValue string                              `bson:"defaultValue,omitempty" json:"defaultValue"`
	Description  string                              `bson:"description,omitempty" json:"description"`
	Enum         []string                            `bson:"enum,omitempty" json:"enum"`
	Properties   map[string]DevopsTektonPropertySpec `bson:"properties,omitempty" json:"properties"`
	Required     bool                                `bson:"required,omitempty" json:"required"`
}

type DevopsTektonTaskResult struct {
	Name        string                              `bson:"name,omitempty" json:"name"`
	Type        string                              `bson:"type,omitempty" json:"type"`
	Description string                              `bson:"description,omitempty" json:"description"`
	Value       string                              `bson:"value,omitempty" json:"value"`
	Properties  map[string]DevopsTektonPropertySpec `bson:"properties,omitempty" json:"properties"`
}

type DevopsTektonWorkspace struct {
	Name        string `bson:"name,omitempty" json:"name"`
	Description string `bson:"description,omitempty" json:"description"`
	Optional    bool   `bson:"optional,omitempty" json:"optional"`
	ReadOnly    bool   `bson:"readOnly,omitempty" json:"readOnly"`
	MountPath   string `bson:"mountPath,omitempty" json:"mountPath"`
}

type DevopsStepParam struct {
	Name             string                  `bson:"name,omitempty" json:"name"`
	Code             string                  `bson:"code,omitempty" json:"code"`
	DefaultValue     string                  `bson:"defaultValue,omitempty" json:"defaultValue"`
	ParamType        string                  `bson:"paramType,omitempty" json:"paramType"`
	Mode             string                  `bson:"mode,omitempty" json:"mode"`
	Required         bool                    `bson:"required,omitempty" json:"required"`
	Readonly         bool                    `bson:"readonly,omitempty" json:"readonly"`
	Description      string                  `bson:"description,omitempty" json:"description"`
	SelectList       []DevopsStepParamOption `bson:"selectList,omitempty" json:"selectList"`
	AllowCustom      bool                    `bson:"allowCustom,omitempty" json:"allowCustom"`
	ChannelGroupCode string                  `bson:"channelGroupCode,omitempty" json:"channelGroupCode"`
	MappingField     string                  `bson:"mappingField,omitempty" json:"mappingField"`
	VoucherModel     string                  `bson:"voucherModel,omitempty" json:"voucherModel"`
	SortOrder        int64                   `bson:"sortOrder,omitempty" json:"sortOrder"`
	RuntimeMode      string                  `bson:"runtimeMode,omitempty" json:"runtimeMode"`
	RuntimeConfig    bool                    `bson:"runtimeConfig,omitempty" json:"runtimeConfig"`
	Config           DevopsStepParamConfig   `bson:"config,omitempty" json:"config"`
}

type DevopsStepTemplate struct {
	ID                     bson.ObjectID            `bson:"_id,omitempty"`
	Name                   string                   `bson:"name,omitempty"`
	Code                   string                   `bson:"code,omitempty"`
	Icon                   string                   `bson:"icon,omitempty"`
	IconColor              string                   `bson:"iconColor,omitempty"`
	Description            string                   `bson:"description,omitempty"`
	Type                   string                   `bson:"type,omitempty"`
	CategoryID             string                   `bson:"categoryId,omitempty"`
	EngineType             string                   `bson:"engineType,omitempty"`
	EngineChannelGroupCode string                   `bson:"engineChannelGroupCode,omitempty"`
	EngineChannelType      string                   `bson:"engineChannelType,omitempty"`
	StageContent           string                   `bson:"stageContent,omitempty"`
	Params                 []DevopsStepParam        `bson:"params,omitempty"`
	TaskParams             []DevopsTektonTaskParam  `bson:"taskParams,omitempty"`
	TaskResults            []DevopsTektonTaskResult `bson:"taskResults,omitempty"`
	TaskWorkspaces         []DevopsTektonWorkspace  `bson:"taskWorkspaces,omitempty"`
	ArtifactConfig         DevopsStepArtifactConfig `bson:"artifactConfig,omitempty"`
	Status                 int64                    `bson:"status"`
	CreatedBy              string                   `bson:"createdBy,omitempty"`
	UpdatedBy              string                   `bson:"updatedBy,omitempty"`
	CreateAt               time.Time                `bson:"createAt,omitempty"`
	UpdateAt               time.Time                `bson:"updateAt,omitempty"`
	IsDeleted              bool                     `bson:"isDeleted"`
}

type DevopsStepTemplateListFilter struct {
	Name       string
	Code       string
	CategoryID string
	EngineType string
	Type       string
	Status     int64
	Page       uint64
	PageSize   uint64
}

type DevopsStepTemplateModel struct {
	conn *mon.Model
}

func NewDevopsStepTemplateModel(url, db string) *DevopsStepTemplateModel {
	return &DevopsStepTemplateModel{
		conn: mon.MustNewModel(url, db, DevopsStepTemplateCollectionName),
	}
}

func (m *DevopsStepTemplateModel) Insert(ctx context.Context, data *DevopsStepTemplate) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsStepTemplateModel) FindOne(ctx context.Context, id string) (*DevopsStepTemplate, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsStepTemplate
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsStepTemplateModel) FindOneByCode(ctx context.Context, engineType, code string) (*DevopsStepTemplate, error) {
	var data DevopsStepTemplate
	err := m.conn.FindOne(ctx, &data, bson.M{"engineType": engineType, "code": code, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsStepTemplateModel) ExistsParamCode(ctx context.Context, engineType, paramCode, excludeID string) (bool, error) {
	query := bson.M{"engineType": engineType, "params.code": paramCode, "isDeleted": false}
	if excludeID != "" {
		oid, err := objectIDFromHex(excludeID)
		if err != nil {
			return false, err
		}
		query["_id"] = bson.M{"$ne": oid}
	}
	total, err := m.conn.CountDocuments(ctx, query)
	if err != nil {
		return false, err
	}
	return total > 0, nil
}

func (m *DevopsStepTemplateModel) CountByCategory(ctx context.Context, categoryID string) (uint64, error) {
	total, err := m.conn.CountDocuments(ctx, bson.M{"categoryId": categoryID, "isDeleted": false})
	if err != nil {
		return 0, err
	}
	return uint64(total), nil
}

func (m *DevopsStepTemplateModel) Update(ctx context.Context, data *DevopsStepTemplate) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"name":                   data.Name,
			"icon":                   data.Icon,
			"iconColor":              data.IconColor,
			"description":            data.Description,
			"type":                   data.Type,
			"categoryId":             data.CategoryID,
			"engineType":             data.EngineType,
			"engineChannelGroupCode": data.EngineChannelGroupCode,
			"engineChannelType":      data.EngineChannelType,
			"stageContent":           data.StageContent,
			"params":                 data.Params,
			"taskParams":             data.TaskParams,
			"taskResults":            data.TaskResults,
			"taskWorkspaces":         data.TaskWorkspaces,
			"artifactConfig":         data.ArtifactConfig,
			"status":                 data.Status,
			"updatedBy":              data.UpdatedBy,
			"updateAt":               data.UpdateAt,
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

func (m *DevopsStepTemplateModel) UpdateStatus(ctx context.Context, id string, status int64, updatedBy string) error {
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

func (m *DevopsStepTemplateModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsStepTemplateModel) List(ctx context.Context, filter DevopsStepTemplateListFilter) ([]*DevopsStepTemplate, uint64, error) {
	query := bson.M{"isDeleted": false}
	if filter.Name != "" {
		query["name"] = bson.M{"$regex": filter.Name, "$options": "i"}
	}
	if filter.Code != "" {
		query["code"] = bson.M{"$regex": filter.Code, "$options": "i"}
	}
	if filter.CategoryID != "" {
		query["categoryId"] = filter.CategoryID
	}
	if filter.EngineType != "" {
		query["engineType"] = filter.EngineType
	}
	if filter.Type != "" {
		query["type"] = filter.Type
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

	var data []*DevopsStepTemplate
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}

func (m *DevopsStepTemplateModel) ListForImageManage(ctx context.Context, filter DevopsStepTemplateListFilter) ([]*DevopsStepTemplate, error) {
	query := bson.M{"isDeleted": false}
	if filter.Name != "" {
		query["name"] = bson.M{"$regex": filter.Name, "$options": "i"}
	}
	if filter.Code != "" {
		query["code"] = bson.M{"$regex": filter.Code, "$options": "i"}
	}
	if filter.CategoryID != "" {
		query["categoryId"] = filter.CategoryID
	}
	if filter.EngineType != "" {
		query["engineType"] = filter.EngineType
	}
	if filter.Type != "" {
		query["type"] = filter.Type
	}
	if filter.Status >= 0 {
		query["status"] = filter.Status
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "createAt", Value: -1}}).
		SetProjection(bson.M{
			"_id":          1,
			"name":         1,
			"code":         1,
			"categoryId":   1,
			"engineType":   1,
			"stageContent": 1,
			"params":       1,
			"taskParams":   1,
			"status":       1,
		})

	var data []*DevopsStepTemplate
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, err
	}
	return data, nil
}
