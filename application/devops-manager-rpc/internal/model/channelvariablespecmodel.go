package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var _ ChannelVariableSpecModel = (*customChannelVariableSpecModel)(nil)

type (
	// ChannelVariableSpecModel 渠道变量规格模型接口
	ChannelVariableSpecModel interface {
		channelVariableSpecModel
		FindBySpecProfile(ctx context.Context, specProfile string) ([]*ChannelVariableSpec, error)
		FindByGroupCode(ctx context.Context, groupCode string) ([]*ChannelVariableSpec, error)
		FindAll(ctx context.Context) ([]*ChannelVariableSpec, error)
	}

	customChannelVariableSpecModel struct {
		*defaultChannelVariableSpecModel
	}

	// ChannelVariableSpec 渠道变量规格
	ChannelVariableSpec struct {
		ID               bson.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
		SpecProfile      string        `bson:"spec_profile" json:"specProfile"`
		FieldKey         string        `bson:"field_key" json:"fieldKey"`
		FieldName        string        `bson:"field_name" json:"fieldName"`
		FieldKind        string        `bson:"field_kind" json:"fieldKind"`
		ProviderKey      string        `bson:"provider_key" json:"providerKey"`
		UIControl        string        `bson:"ui_control" json:"uiControl"`
		Dependencies     interface{}   `bson:"dependencies" json:"dependencies"`
		AllowManualInput bool          `bson:"allow_manual_input" json:"allowManualInput"`
		IsRequired       bool          `bson:"is_required" json:"isRequired"`
		DefaultValue     string        `bson:"default_value" json:"defaultValue"`
		Placeholder      string        `bson:"placeholder" json:"placeholder"`
		HelpText         string        `bson:"help_text" json:"helpText"`
		SortOrder        int64         `bson:"sort_order" json:"sortOrder"`
		CreatedBy        string        `bson:"created_by" json:"createdBy"`
		UpdatedBy        string        `bson:"updated_by" json:"updatedBy"`
		CreatedAt        time.Time     `bson:"created_at" json:"createdAt"`
		UpdatedAt        time.Time     `bson:"updated_at" json:"updatedAt"`
		IsDeleted        bool          `bson:"is_deleted" json:"isDeleted"`
	}

	channelVariableSpecModel interface {
		Insert(ctx context.Context, data *ChannelVariableSpec) error
		FindOne(ctx context.Context, id string) (*ChannelVariableSpec, error)
		Update(ctx context.Context, data *ChannelVariableSpec) error
		Delete(ctx context.Context, id string) error
	}

	defaultChannelVariableSpecModel struct {
		conn *mon.Model
	}
)

// NewChannelVariableSpecModel 创建渠道变量规格模型
func NewChannelVariableSpecModel(url, db string) ChannelVariableSpecModel {
	conn := mon.MustNewModel(url, db, "channel_variable_spec")
	return &customChannelVariableSpecModel{
		defaultChannelVariableSpecModel: newChannelVariableSpecModel(conn),
	}
}

func newChannelVariableSpecModel(conn *mon.Model) *defaultChannelVariableSpecModel {
	return &defaultChannelVariableSpecModel{
		conn: conn,
	}
}

func (m *defaultChannelVariableSpecModel) Insert(ctx context.Context, data *ChannelVariableSpec) error {
	if data.ID.IsZero() {
		data.ID = bson.NewObjectID()
	}
	data.CreatedAt = time.Now()
	data.UpdatedAt = time.Now()

	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *defaultChannelVariableSpecModel) FindOne(ctx context.Context, id string) (*ChannelVariableSpec, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, ErrInvalidObjectId
	}

	var data ChannelVariableSpec
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "is_deleted": false})
	switch err {
	case nil:
		return &data, nil
	case mon.ErrNotFound:
		return nil, ErrNotFoundChannelVariableSpec
	default:
		return nil, err
	}
}

func (m *defaultChannelVariableSpecModel) Update(ctx context.Context, data *ChannelVariableSpec) error {
	data.UpdatedAt = time.Now()

	_, err := m.conn.UpdateOne(ctx, bson.M{"_id": data.ID}, bson.M{"$set": data})
	return err
}

func (m *defaultChannelVariableSpecModel) Delete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return ErrInvalidObjectId
	}

	_, err = m.conn.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{"is_deleted": true, "updated_at": time.Now()}})
	return err
}

// FindBySpecProfile 根据规格模板查询
func (m *customChannelVariableSpecModel) FindBySpecProfile(ctx context.Context, specProfile string) ([]*ChannelVariableSpec, error) {
	var data []*ChannelVariableSpec
	err := m.conn.Find(ctx, &data, bson.M{"spec_profile": specProfile, "is_deleted": false}, options.Find().SetSort(bson.M{"sort_order": 1}))
	switch err {
	case nil:
		return data, nil
	case mon.ErrNotFound:
		return nil, ErrNotFoundChannelVariableSpec
	default:
		return nil, err
	}
}

// FindByGroupCode 根据分组查询
func (m *customChannelVariableSpecModel) FindByGroupCode(ctx context.Context, groupCode string) ([]*ChannelVariableSpec, error) {
	// 暂时返回空列表，因为DevopsChannelType还没有SpecProfile字段
	// TODO: 在DevopsChannelType添加SpecProfile和ProviderType字段后实现此方法
	return []*ChannelVariableSpec{}, nil
}

// FindAll 查询所有规格
func (m *customChannelVariableSpecModel) FindAll(ctx context.Context) ([]*ChannelVariableSpec, error) {
	var data []*ChannelVariableSpec
	err := m.conn.Find(ctx, &data, bson.M{"is_deleted": false}, options.Find().SetSort(bson.M{"spec_profile": 1, "sort_order": 1}))
	switch err {
	case nil:
		return data, nil
	case mon.ErrNotFound:
		return nil, ErrNotFoundChannelVariableSpec
	default:
		return nil, err
	}
}

var (
	ErrNotFoundChannelVariableSpec = mon.ErrNotFound
	ErrInvalidObjectId             = ErrNotFoundChannelVariableSpec
)
