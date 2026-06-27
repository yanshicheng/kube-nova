package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsStepChannelParamCollectionName = "devops_step_channel_param"

// DevopsStepChannelParam 步骤参数类型到渠道分组的映射。
// 替代 channelvars 包中的硬编码 switch-case，将 paramType → groupCode 映射改为数据库驱动。
type DevopsStepChannelParam struct {
	ID                bson.ObjectID `bson:"_id,omitempty"`
	ParamType         string        `bson:"paramType"`         // 步骤参数类型，如 repositoryChannel、harborChannel
	ParamName         string        `bson:"paramName"`         // 参数中文名称
	GroupCode         string        `bson:"groupCode"`         // 关联的渠道分组编码
	ChannelTypeFilter string        `bson:"channelTypeFilter"` // 限制渠道类型，空表示分组下所有类型均可
	Description       string        `bson:"description"`
	SortOrder         int64         `bson:"sortOrder"`
	IsSystem          bool          `bson:"isSystem"`
	Status            int64         `bson:"status"`
	CreatedBy         string        `bson:"createdBy"`
	UpdatedBy         string        `bson:"updatedBy"`
	CreateAt          time.Time     `bson:"createAt"`
	UpdateAt          time.Time     `bson:"updateAt"`
	IsDeleted         bool          `bson:"isDeleted"`
}

// StepChannelParamSpec 供 channelvars 动态查询使用的只读结果。
type StepChannelParamSpec struct {
	ParamType         string
	GroupCode         string
	ChannelTypeFilter string
}

type DevopsStepChannelParamModel struct {
	conn *mon.Model
}

func NewDevopsStepChannelParamModel(url, db string) *DevopsStepChannelParamModel {
	return &DevopsStepChannelParamModel{
		conn: mon.MustNewModel(url, db, DevopsStepChannelParamCollectionName),
	}
}

func (m *DevopsStepChannelParamModel) Insert(ctx context.Context, data *DevopsStepChannelParam) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsStepChannelParamModel) FindOne(ctx context.Context, id string) (*DevopsStepChannelParam, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsStepChannelParam
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsStepChannelParamModel) FindOneByParamType(ctx context.Context, paramType string) (*DevopsStepChannelParam, error) {
	var data DevopsStepChannelParam
	err := m.conn.FindOne(ctx, &data, bson.M{"paramType": paramType, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsStepChannelParamModel) FindOneByTarget(ctx context.Context, paramType, groupCode, channelTypeFilter string) (*DevopsStepChannelParam, error) {
	var data DevopsStepChannelParam
	err := m.conn.FindOne(ctx, &data, bson.M{
		"paramType":         paramType,
		"groupCode":         groupCode,
		"channelTypeFilter": channelTypeFilter,
		"isDeleted":         false,
	})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

// LoadActiveSpecs 加载所有启用状态的映射，供内存缓存使用。
func (m *DevopsStepChannelParamModel) LoadActiveSpecs(ctx context.Context) ([]StepChannelParamSpec, error) {
	var data []*DevopsStepChannelParam
	err := m.conn.Find(ctx, &data, bson.M{"isDeleted": false, "status": int64(1)})
	if err != nil {
		return nil, err
	}
	result := make([]StepChannelParamSpec, 0, len(data))
	for _, item := range data {
		if item == nil {
			continue
		}
		result = append(result, StepChannelParamSpec{
			ParamType:         item.ParamType,
			GroupCode:         item.GroupCode,
			ChannelTypeFilter: item.ChannelTypeFilter,
		})
	}
	return result, nil
}

func (m *DevopsStepChannelParamModel) Update(ctx context.Context, data *DevopsStepChannelParam) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"paramType":         data.ParamType,
			"paramName":         data.ParamName,
			"groupCode":         data.GroupCode,
			"channelTypeFilter": data.ChannelTypeFilter,
			"description":       data.Description,
			"sortOrder":         data.SortOrder,
			"status":            data.Status,
			"updatedBy":         data.UpdatedBy,
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

func (m *DevopsStepChannelParamModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsStepChannelParamModel) DeleteSystemDefaults(ctx context.Context, updatedBy string) error {
	_, err := m.conn.UpdateMany(ctx,
		bson.M{"isSystem": true, "isDeleted": false},
		bson.M{"$set": bson.M{"isDeleted": true, "updatedBy": updatedBy, "updateAt": now()}},
	)
	return err
}

func (m *DevopsStepChannelParamModel) CountByParamType(ctx context.Context, paramType string) (int64, error) {
	return m.conn.CountDocuments(ctx, bson.M{"paramType": paramType, "isDeleted": false})
}

func (m *DevopsStepChannelParamModel) CountByTarget(ctx context.Context, paramType, groupCode, channelTypeFilter string) (int64, error) {
	return m.conn.CountDocuments(ctx, bson.M{
		"paramType":         paramType,
		"groupCode":         groupCode,
		"channelTypeFilter": channelTypeFilter,
		"isDeleted":         false,
	})
}

func (m *DevopsStepChannelParamModel) List(ctx context.Context, filter DevopsStepChannelParamListFilter) ([]*DevopsStepChannelParam, uint64, error) {
	query := bson.M{"isDeleted": false}
	if filter.ParamType != "" {
		query["paramType"] = bson.M{"$regex": filter.ParamType, "$options": "i"}
	}
	if filter.GroupCode != "" {
		query["groupCode"] = filter.GroupCode
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
		SetSort(bson.D{{Key: "sortOrder", Value: 1}, {Key: "createAt", Value: -1}}).
		SetSkip(int64((page.Page - 1) * page.PageSize)).
		SetLimit(int64(page.PageSize))

	var data []*DevopsStepChannelParam
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}

type DevopsStepChannelParamListFilter struct {
	ParamType string
	GroupCode string
	Status    int64
	Page      uint64
	PageSize  uint64
}
