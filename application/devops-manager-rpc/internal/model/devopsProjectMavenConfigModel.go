package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsProjectMavenConfigCollectionName = "devops_project_maven_config"

type DevopsProjectMavenConfig struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	ProjectID   string        `bson:"projectId,omitempty"`
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

type DevopsProjectMavenConfigListFilter struct {
	ProjectID  string
	ProjectIDs []string
	Restricted bool
	Name       string
	Code       string
	Status     int64
	Page       uint64
	PageSize   uint64
}

type DevopsProjectMavenConfigModel struct {
	conn *mon.Model
}

func NewDevopsProjectMavenConfigModel(url, db string) *DevopsProjectMavenConfigModel {
	return &DevopsProjectMavenConfigModel{
		conn: mon.MustNewModel(url, db, DevopsProjectMavenConfigCollectionName),
	}
}

func (m *DevopsProjectMavenConfigModel) Insert(ctx context.Context, data *DevopsProjectMavenConfig) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsProjectMavenConfigModel) FindOne(ctx context.Context, id string) (*DevopsProjectMavenConfig, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsProjectMavenConfig
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsProjectMavenConfigModel) FindOneByProjectCode(ctx context.Context, projectID, code string) (*DevopsProjectMavenConfig, error) {
	var data DevopsProjectMavenConfig
	err := m.conn.FindOne(ctx, &data, bson.M{"projectId": projectID, "code": code, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsProjectMavenConfigModel) Update(ctx context.Context, data *DevopsProjectMavenConfig) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
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

func (m *DevopsProjectMavenConfigModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsProjectMavenConfigModel) DeleteSoftByProject(ctx context.Context, projectID, updatedBy string) error {
	_, err := m.conn.UpdateMany(ctx,
		bson.M{"projectId": projectID, "isDeleted": false},
		bson.M{"$set": bson.M{"isDeleted": true, "updatedBy": updatedBy, "updateAt": now()}},
	)
	return err
}

func (m *DevopsProjectMavenConfigModel) CountByProjectCode(ctx context.Context, projectID, code, excludeID string) (uint64, error) {
	query := bson.M{"projectId": projectID, "code": code, "isDeleted": false}
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

func (m *DevopsProjectMavenConfigModel) CountByProjectName(ctx context.Context, projectID, name, excludeID string) (uint64, error) {
	query := bson.M{"projectId": projectID, "name": name, "isDeleted": false}
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

func (m *DevopsProjectMavenConfigModel) List(ctx context.Context, filter DevopsProjectMavenConfigListFilter) ([]*DevopsProjectMavenConfig, uint64, error) {
	query := bson.M{"isDeleted": false}
	if filter.Restricted {
		if len(filter.ProjectIDs) == 0 {
			return []*DevopsProjectMavenConfig{}, 0, nil
		}
		query["projectId"] = bson.M{"$in": filter.ProjectIDs}
	}
	if filter.ProjectID != "" {
		query["projectId"] = filter.ProjectID
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

	var data []*DevopsProjectMavenConfig
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}
