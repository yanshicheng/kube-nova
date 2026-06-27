package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsProjectCollectionName = "devops_project"

type DevopsProject struct {
	ID                     bson.ObjectID `bson:"_id,omitempty"`
	Name                   string        `bson:"name,omitempty"`
	Code                   string        `bson:"code,omitempty"`
	Description            string        `bson:"description,omitempty"`
	PipelineEngineType     string        `bson:"pipelineEngineType,omitempty"`
	DefaultEngineChannelID string        `bson:"defaultEngineChannelId,omitempty"`
	Status                 int64         `bson:"status"`
	ExtraConfig            string        `bson:"extraConfig,omitempty"`
	CreatedBy              string        `bson:"createdBy,omitempty"`
	UpdatedBy              string        `bson:"updatedBy,omitempty"`
	CreateAt               time.Time     `bson:"createAt,omitempty"`
	UpdateAt               time.Time     `bson:"updateAt,omitempty"`
	IsDeleted              bool          `bson:"isDeleted"`
}

type DevopsProjectListFilter struct {
	Name               string
	Code               string
	PipelineEngineType string
	Status             int64
	ProjectIDs         []string
	Restricted         bool
	Page               uint64
	PageSize           uint64
}

type DevopsProjectModel struct {
	conn *mon.Model
}

func NewDevopsProjectModel(url, db string) *DevopsProjectModel {
	return &DevopsProjectModel{
		conn: mon.MustNewModel(url, db, DevopsProjectCollectionName),
	}
}

func (m *DevopsProjectModel) Insert(ctx context.Context, data *DevopsProject) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsProjectModel) FindOne(ctx context.Context, id string) (*DevopsProject, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsProject
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsProjectModel) Update(ctx context.Context, data *DevopsProject) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"name":                   data.Name,
			"description":            data.Description,
			"pipelineEngineType":     data.PipelineEngineType,
			"defaultEngineChannelId": data.DefaultEngineChannelID,
			"status":                 data.Status,
			"extraConfig":            data.ExtraConfig,
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

func (m *DevopsProjectModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsProjectModel) List(ctx context.Context, filter DevopsProjectListFilter) ([]*DevopsProject, uint64, error) {
	query := bson.M{"isDeleted": false}
	if filter.Restricted {
		if len(filter.ProjectIDs) == 0 {
			return []*DevopsProject{}, 0, nil
		}
		objectIDs := make([]bson.ObjectID, 0, len(filter.ProjectIDs))
		for _, id := range filter.ProjectIDs {
			oid, err := objectIDFromHex(id)
			if err != nil {
				continue
			}
			objectIDs = append(objectIDs, oid)
		}
		if len(objectIDs) == 0 {
			return []*DevopsProject{}, 0, nil
		}
		query["_id"] = bson.M{"$in": objectIDs}
	}
	if filter.Name != "" {
		query["name"] = bson.M{"$regex": filter.Name, "$options": "i"}
	}
	if filter.Code != "" {
		query["code"] = bson.M{"$regex": filter.Code, "$options": "i"}
	}
	if filter.PipelineEngineType != "" {
		query["pipelineEngineType"] = filter.PipelineEngineType
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

	var data []*DevopsProject
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}
