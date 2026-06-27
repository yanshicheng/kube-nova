package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsChannelTypeCollectionName = "devops_channel_type"

type DevopsChannelType struct {
	ID               bson.ObjectID `bson:"_id,omitempty"`
	Code             string        `bson:"code,omitempty"`
	Name             string        `bson:"name,omitempty"`
	GroupCode        string        `bson:"groupCode,omitempty"`
	CredentialTypes  []string      `bson:"credentialTypes,omitempty"`
	ConfigSchema     string        `bson:"configSchema,omitempty"`
	MappingFields    string        `bson:"mappingFields,omitempty"`
	TestStrategy     string        `bson:"testStrategy,omitempty"`
	Icon             string        `bson:"icon,omitempty"`
	IconColor        string        `bson:"iconColor,omitempty"`
	ConnectionMode   string        `bson:"connectionMode,omitempty"`
	MetadataStrategy string        `bson:"metadataStrategy,omitempty"`
	IsSystem         bool          `bson:"isSystem,omitempty"`
	Status           int64         `bson:"status"`
	CreatedBy        string        `bson:"createdBy,omitempty"`
	UpdatedBy        string        `bson:"updatedBy,omitempty"`
	CreateAt         time.Time     `bson:"createAt,omitempty"`
	UpdateAt         time.Time     `bson:"updateAt,omitempty"`
	IsDeleted        bool          `bson:"isDeleted"`
}

type DevopsChannelTypeListFilter struct {
	Name      string
	Code      string
	GroupCode string
	Status    int64
	Page      uint64
	PageSize  uint64
}

type DevopsChannelTypeModel struct {
	conn *mon.Model
}

func NewDevopsChannelTypeModel(url, db string) *DevopsChannelTypeModel {
	return &DevopsChannelTypeModel{
		conn: mon.MustNewModel(url, db, DevopsChannelTypeCollectionName),
	}
}

func (m *DevopsChannelTypeModel) Insert(ctx context.Context, data *DevopsChannelType) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsChannelTypeModel) FindOne(ctx context.Context, id string) (*DevopsChannelType, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsChannelType
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsChannelTypeModel) FindOneByCode(ctx context.Context, code string) (*DevopsChannelType, error) {
	var data DevopsChannelType
	err := m.conn.FindOne(ctx, &data, bson.M{"code": code, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsChannelTypeModel) MarkSystem(ctx context.Context, id string, data *DevopsChannelType) error {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = m.conn.UpdateOne(ctx,
		bson.M{"_id": oid, "isDeleted": false},
		bson.M{"$set": bson.M{
			"name":             data.Name,
			"groupCode":        data.GroupCode,
			"credentialTypes":  data.CredentialTypes,
			"configSchema":     data.ConfigSchema,
			"mappingFields":    data.MappingFields,
			"testStrategy":     data.TestStrategy,
			"icon":             data.Icon,
			"iconColor":        data.IconColor,
			"connectionMode":   data.ConnectionMode,
			"metadataStrategy": data.MetadataStrategy,
			"isSystem":         true,
			"status":           data.Status,
			"updatedBy":        data.UpdatedBy,
			"updateAt":         now(),
		}},
	)
	return err
}

func (m *DevopsChannelTypeModel) Update(ctx context.Context, data *DevopsChannelType) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"name":             data.Name,
			"groupCode":        data.GroupCode,
			"credentialTypes":  data.CredentialTypes,
			"configSchema":     data.ConfigSchema,
			"mappingFields":    data.MappingFields,
			"testStrategy":     data.TestStrategy,
			"icon":             data.Icon,
			"iconColor":        data.IconColor,
			"connectionMode":   data.ConnectionMode,
			"metadataStrategy": data.MetadataStrategy,
			"status":           data.Status,
			"updatedBy":        data.UpdatedBy,
			"updateAt":         data.UpdateAt,
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

func (m *DevopsChannelTypeModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsChannelTypeModel) DisableByCode(ctx context.Context, code, updatedBy string) error {
	_, err := m.conn.UpdateMany(ctx,
		bson.M{"code": code, "isDeleted": false},
		bson.M{"$set": bson.M{"status": int64(0), "updatedBy": updatedBy, "updateAt": now()}},
	)
	return err
}

func (m *DevopsChannelTypeModel) List(ctx context.Context, filter DevopsChannelTypeListFilter) ([]*DevopsChannelType, uint64, error) {
	query := bson.M{"isDeleted": false}
	if filter.Name != "" {
		query["name"] = bson.M{"$regex": filter.Name, "$options": "i"}
	}
	if filter.Code != "" {
		query["code"] = bson.M{"$regex": filter.Code, "$options": "i"}
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
		SetSort(bson.D{{Key: "groupCode", Value: 1}, {Key: "code", Value: 1}}).
		SetSkip(int64((page.Page - 1) * page.PageSize)).
		SetLimit(int64(page.PageSize))

	var data []*DevopsChannelType
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}
