package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsProjectConfigCollectionName = "devops_project_config"

type DevopsProjectConfig struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	ProjectID   string        `bson:"projectId,omitempty"`
	TypeID      string        `bson:"typeId,omitempty"`
	TypeCode    string        `bson:"typeCode,omitempty"`
	TypeName    string        `bson:"typeName,omitempty"`
	Name        string        `bson:"name,omitempty"`
	Code        string        `bson:"code,omitempty"`
	Content     string        `bson:"content,omitempty"`
	Description string        `bson:"description,omitempty"`
	Status      int64         `bson:"status"`
	CreatedBy   string        `bson:"createdBy,omitempty"`
	UpdatedBy   string        `bson:"updatedBy,omitempty"`
	CreateAt    time.Time     `bson:"createAt,omitempty"`
	UpdateAt    time.Time     `bson:"updateAt,omitempty"`
	IsDeleted   bool          `bson:"isDeleted"`
}

type DevopsProjectConfigListFilter struct {
	ProjectID  string
	ProjectIDs []string
	Restricted bool
	TypeID     string
	TypeCode   string
	Name       string
	Code       string
	Status     int64
	Page       uint64
	PageSize   uint64
}

type DevopsProjectConfigModel struct {
	conn *mon.Model
}

func NewDevopsProjectConfigModel(url, db string) *DevopsProjectConfigModel {
	return &DevopsProjectConfigModel{
		conn: mon.MustNewModel(url, db, DevopsProjectConfigCollectionName),
	}
}

func (m *DevopsProjectConfigModel) Insert(ctx context.Context, data *DevopsProjectConfig) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsProjectConfigModel) FindOne(ctx context.Context, id string) (*DevopsProjectConfig, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsProjectConfig
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsProjectConfigModel) FindOneByProjectTypeCode(ctx context.Context, projectID, typeCode, code string) (*DevopsProjectConfig, error) {
	var data DevopsProjectConfig
	err := m.conn.FindOne(ctx, &data, bson.M{
		"projectId": projectID,
		"typeCode":  typeCode,
		"code":      code,
		"isDeleted": false,
	})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsProjectConfigModel) Update(ctx context.Context, data *DevopsProjectConfig) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"typeId":      data.TypeID,
			"typeCode":    data.TypeCode,
			"typeName":    data.TypeName,
			"name":        data.Name,
			"code":        data.Code,
			"content":     data.Content,
			"description": data.Description,
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

func (m *DevopsProjectConfigModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsProjectConfigModel) DeleteSoftByProject(ctx context.Context, projectID, updatedBy string) error {
	_, err := m.conn.UpdateMany(ctx,
		bson.M{"projectId": projectID, "isDeleted": false},
		bson.M{"$set": bson.M{"isDeleted": true, "updatedBy": updatedBy, "updateAt": now()}},
	)
	return err
}

func (m *DevopsProjectConfigModel) CountByProjectTypeCode(ctx context.Context, projectID, typeCode, code, excludeID string) (uint64, error) {
	query := bson.M{"projectId": projectID, "typeCode": typeCode, "code": code, "isDeleted": false}
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

func (m *DevopsProjectConfigModel) CountByProjectTypeName(ctx context.Context, projectID, typeCode, name, excludeID string) (uint64, error) {
	query := bson.M{"projectId": projectID, "typeCode": typeCode, "name": name, "isDeleted": false}
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

func (m *DevopsProjectConfigModel) CountByTypeCode(ctx context.Context, typeCode string) (uint64, error) {
	total, err := m.conn.CountDocuments(ctx, bson.M{"typeCode": typeCode, "isDeleted": false})
	return uint64(total), err
}

func (m *DevopsProjectConfigModel) List(ctx context.Context, filter DevopsProjectConfigListFilter) ([]*DevopsProjectConfig, uint64, error) {
	query := bson.M{"isDeleted": false}
	if filter.Restricted {
		if len(filter.ProjectIDs) == 0 {
			return []*DevopsProjectConfig{}, 0, nil
		}
		query["projectId"] = bson.M{"$in": filter.ProjectIDs}
	}
	if filter.ProjectID != "" {
		query["projectId"] = filter.ProjectID
	}
	if filter.TypeID != "" {
		query["typeId"] = filter.TypeID
	}
	if filter.TypeCode != "" {
		query["typeCode"] = filter.TypeCode
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
		SetSort(bson.D{{Key: "createAt", Value: -1}}).
		SetSkip(int64((page.Page - 1) * page.PageSize)).
		SetLimit(int64(page.PageSize))

	var data []*DevopsProjectConfig
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}
