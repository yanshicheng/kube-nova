package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsConfigTypeCollectionName = "devops_config_type"
const DefaultMavenSettingsTypeCode = "MavenSettings"

type DevopsConfigType struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	ParentID    string        `bson:"parentId,omitempty"`
	Name        string        `bson:"name,omitempty"`
	Code        string        `bson:"code,omitempty"`
	StorageType string        `bson:"storageType,omitempty"`
	Description string        `bson:"description,omitempty"`
	SortOrder   int64         `bson:"sortOrder,omitempty"`
	Status      int64         `bson:"status"`
	CreatedBy   string        `bson:"createdBy,omitempty"`
	UpdatedBy   string        `bson:"updatedBy,omitempty"`
	CreateAt    time.Time     `bson:"createAt,omitempty"`
	UpdateAt    time.Time     `bson:"updateAt,omitempty"`
	IsDeleted   bool          `bson:"isDeleted"`
}

type DevopsConfigTypeListFilter struct {
	ParentID string
	Name     string
	Code     string
	Status   int64
	Page     uint64
	PageSize uint64
}

type DevopsConfigTypeModel struct {
	conn *mon.Model
}

func NewDevopsConfigTypeModel(url, db string) *DevopsConfigTypeModel {
	return &DevopsConfigTypeModel{
		conn: mon.MustNewModel(url, db, DevopsConfigTypeCollectionName),
	}
}

func (m *DevopsConfigTypeModel) Insert(ctx context.Context, data *DevopsConfigType) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsConfigTypeModel) FindOne(ctx context.Context, id string) (*DevopsConfigType, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsConfigType
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsConfigTypeModel) FindOneByCode(ctx context.Context, code string) (*DevopsConfigType, error) {
	var data DevopsConfigType
	err := m.conn.FindOne(ctx, &data, bson.M{"code": code, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsConfigTypeModel) Update(ctx context.Context, data *DevopsConfigType) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"parentId":    data.ParentID,
			"name":        data.Name,
			"storageType": data.StorageType,
			"description": data.Description,
			"sortOrder":   data.SortOrder,
			"status":      data.Status,
			"updatedBy":   data.UpdatedBy,
			"updateAt":    data.UpdateAt,
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

func (m *DevopsConfigTypeModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsConfigTypeModel) CountByCode(ctx context.Context, code, excludeID string) (uint64, error) {
	query := bson.M{"code": code, "isDeleted": false}
	if excludeID != "" {
		oid, err := objectIDFromHex(excludeID)
		if err != nil {
			return 0, err
		}
		query["_id"] = bson.M{"$ne": oid}
	}
	total, err := m.conn.CountDocuments(ctx, query)
	return uint64(total), err
}

func (m *DevopsConfigTypeModel) CountByParentName(ctx context.Context, parentID, name, excludeID string) (uint64, error) {
	query := bson.M{"parentId": parentID, "name": name, "isDeleted": false}
	if excludeID != "" {
		oid, err := objectIDFromHex(excludeID)
		if err != nil {
			return 0, err
		}
		query["_id"] = bson.M{"$ne": oid}
	}
	total, err := m.conn.CountDocuments(ctx, query)
	return uint64(total), err
}

func (m *DevopsConfigTypeModel) CountByParent(ctx context.Context, parentID string) (uint64, error) {
	total, err := m.conn.CountDocuments(ctx, bson.M{"parentId": parentID, "isDeleted": false})
	return uint64(total), err
}

func (m *DevopsConfigTypeModel) List(ctx context.Context, filter DevopsConfigTypeListFilter) ([]*DevopsConfigType, uint64, error) {
	query := bson.M{"isDeleted": false}
	if filter.ParentID != "" {
		query["parentId"] = filter.ParentID
	}
	if filter.Name != "" {
		query["name"] = bson.M{"$regex": filter.Name, "$options": "i"}
	}
	if filter.Code != "" {
		query["code"] = bson.M{"$regex": filter.Code, "$options": "i"}
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

	var data []*DevopsConfigType
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}

func (m *DevopsConfigTypeModel) All(ctx context.Context, status int64) ([]*DevopsConfigType, error) {
	query := bson.M{"isDeleted": false}
	if status >= 0 {
		query["status"] = status
	}
	opts := options.Find().SetSort(bson.D{{Key: "sortOrder", Value: 1}, {Key: "createAt", Value: -1}})
	var data []*DevopsConfigType
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, err
	}
	return data, nil
}
