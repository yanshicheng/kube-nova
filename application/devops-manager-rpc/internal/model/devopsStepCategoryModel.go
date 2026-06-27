package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsStepCategoryCollectionName = "devops_step_category"

type DevopsStepCategory struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	Name        string        `bson:"name,omitempty"`
	Code        string        `bson:"code,omitempty"`
	Description string        `bson:"description,omitempty"`
	Icon        string        `bson:"icon,omitempty"`
	IconColor   string        `bson:"iconColor,omitempty"`
	SortOrder   int64         `bson:"sortOrder,omitempty"`
	Status      int64         `bson:"status"`
	CreatedBy   string        `bson:"createdBy,omitempty"`
	UpdatedBy   string        `bson:"updatedBy,omitempty"`
	CreateAt    time.Time     `bson:"createAt,omitempty"`
	UpdateAt    time.Time     `bson:"updateAt,omitempty"`
	IsDeleted   bool          `bson:"isDeleted"`
}

type DevopsStepCategoryListFilter struct {
	Name     string
	Code     string
	Status   int64
	Page     uint64
	PageSize uint64
}

type DevopsStepCategoryModel struct {
	conn *mon.Model
}

func NewDevopsStepCategoryModel(url, db string) *DevopsStepCategoryModel {
	return &DevopsStepCategoryModel{
		conn: mon.MustNewModel(url, db, DevopsStepCategoryCollectionName),
	}
}

func (m *DevopsStepCategoryModel) Insert(ctx context.Context, data *DevopsStepCategory) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsStepCategoryModel) FindOne(ctx context.Context, id string) (*DevopsStepCategory, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsStepCategory
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsStepCategoryModel) FindOneByCode(ctx context.Context, code string) (*DevopsStepCategory, error) {
	var data DevopsStepCategory
	err := m.conn.FindOne(ctx, &data, bson.M{"code": code, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsStepCategoryModel) FindByIDs(ctx context.Context, ids []string) (map[string]*DevopsStepCategory, error) {
	objectIDs := make([]bson.ObjectID, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		oid, err := objectIDFromHex(id)
		if err != nil {
			continue
		}
		seen[id] = struct{}{}
		objectIDs = append(objectIDs, oid)
	}
	if len(objectIDs) == 0 {
		return map[string]*DevopsStepCategory{}, nil
	}
	var data []*DevopsStepCategory
	if err := m.conn.Find(ctx, &data, bson.M{
		"_id":       bson.M{"$in": objectIDs},
		"isDeleted": false,
	}); err != nil {
		return nil, err
	}
	result := make(map[string]*DevopsStepCategory, len(data))
	for _, item := range data {
		if item == nil {
			continue
		}
		result[item.ID.Hex()] = item
	}
	return result, nil
}

func (m *DevopsStepCategoryModel) Update(ctx context.Context, data *DevopsStepCategory) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"name":        data.Name,
			"description": data.Description,
			"icon":        data.Icon,
			"iconColor":   data.IconColor,
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

func (m *DevopsStepCategoryModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsStepCategoryModel) List(ctx context.Context, filter DevopsStepCategoryListFilter) ([]*DevopsStepCategory, uint64, error) {
	query := bson.M{"isDeleted": false}
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

	var data []*DevopsStepCategory
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}
