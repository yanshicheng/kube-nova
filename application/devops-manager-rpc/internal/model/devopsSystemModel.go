package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsSystemCollectionName = "devops_system"

type DevopsSystem struct {
	ID           bson.ObjectID `bson:"_id,omitempty"`
	ProjectID    string        `bson:"projectId,omitempty"`
	Name         string        `bson:"name,omitempty"`
	Code         string        `bson:"code,omitempty"`
	Description  string        `bson:"description,omitempty"`
	OwnerUserIDs []uint64      `bson:"ownerUserIds,omitempty"`
	Status       int64         `bson:"status"`
	CreatedBy    string        `bson:"createdBy,omitempty"`
	UpdatedBy    string        `bson:"updatedBy,omitempty"`
	CreateAt     time.Time     `bson:"createAt,omitempty"`
	UpdateAt     time.Time     `bson:"updateAt,omitempty"`
	IsDeleted    bool          `bson:"isDeleted"`
}

type DevopsSystemListFilter struct {
	ProjectID  string
	ProjectIDs []string
	Restricted bool
	Name       string
	Code       string
	Status     int64
	Page       uint64
	PageSize   uint64
}

type DevopsSystemModel struct {
	conn *mon.Model
}

func NewDevopsSystemModel(url, db string) *DevopsSystemModel {
	return &DevopsSystemModel{
		conn: mon.MustNewModel(url, db, DevopsSystemCollectionName),
	}
}

func (m *DevopsSystemModel) Insert(ctx context.Context, data *DevopsSystem) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsSystemModel) FindOne(ctx context.Context, id string) (*DevopsSystem, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsSystem
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsSystemModel) FindOneByProjectCode(ctx context.Context, projectID, code string) (*DevopsSystem, error) {
	var data DevopsSystem
	err := m.conn.FindOne(ctx, &data, bson.M{
		"projectId": projectID,
		"code":      code,
		"isDeleted": false,
	})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsSystemModel) Update(ctx context.Context, data *DevopsSystem) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"name":         data.Name,
			"description":  data.Description,
			"ownerUserIds": data.OwnerUserIDs,
			"status":       data.Status,
			"updatedBy":    data.UpdatedBy,
			"updateAt":     data.UpdateAt,
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

func (m *DevopsSystemModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsSystemModel) List(ctx context.Context, filter DevopsSystemListFilter) ([]*DevopsSystem, uint64, error) {
	query := bson.M{"isDeleted": false}
	if filter.ProjectID != "" {
		if filter.Restricted && !stringSliceContains(filter.ProjectIDs, filter.ProjectID) {
			return []*DevopsSystem{}, 0, nil
		}
		query["projectId"] = filter.ProjectID
	} else if filter.Restricted {
		if len(filter.ProjectIDs) == 0 {
			return []*DevopsSystem{}, 0, nil
		}
		query["projectId"] = bson.M{"$in": filter.ProjectIDs}
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
		SetSort(bson.D{{Key: "projectId", Value: 1}, {Key: "createAt", Value: -1}}).
		SetSkip(int64((page.Page - 1) * page.PageSize)).
		SetLimit(int64(page.PageSize))

	var data []*DevopsSystem
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}
