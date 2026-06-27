package model

import (
	"context"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsBootstrapStateCollectionName = "devops_bootstrap_state"

type DevopsBootstrapState struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	Code        string        `bson:"code,omitempty"`
	Version     string        `bson:"version,omitempty"`
	Description string        `bson:"description,omitempty"`
	Status      int64         `bson:"status"`
	CreatedBy   string        `bson:"createdBy,omitempty"`
	UpdatedBy   string        `bson:"updatedBy,omitempty"`
	CreateAt    time.Time     `bson:"createAt,omitempty"`
	UpdateAt    time.Time     `bson:"updateAt,omitempty"`
	IsDeleted   bool          `bson:"isDeleted"`
}

type DevopsBootstrapStateModel struct {
	conn *mon.Model
}

func NewDevopsBootstrapStateModel(url, db string) *DevopsBootstrapStateModel {
	return &DevopsBootstrapStateModel{
		conn: mon.MustNewModel(url, db, DevopsBootstrapStateCollectionName),
	}
}

func (m *DevopsBootstrapStateModel) FindOneByCode(ctx context.Context, code string) (*DevopsBootstrapState, error) {
	var data DevopsBootstrapState
	err := m.conn.FindOne(ctx, &data, bson.M{"code": code, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsBootstrapStateModel) IsVersionApplied(ctx context.Context, code, version string) (bool, error) {
	data, err := m.FindOneByCode(ctx, code)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return data.Status == 1 && data.Version == version, nil
}

func (m *DevopsBootstrapStateModel) MarkVersionApplied(ctx context.Context, code, version, description, updatedBy string) error {
	t := now()
	_, err := m.conn.UpdateOne(ctx,
		bson.M{"code": code, "isDeleted": false},
		bson.M{
			"$setOnInsert": bson.M{
				"_id":       bson.NewObjectID(),
				"code":      code,
				"createdBy": updatedBy,
				"createAt":  t,
				"isDeleted": false,
			},
			"$set": bson.M{
				"version":     version,
				"description": description,
				"status":      int64(1),
				"updatedBy":   updatedBy,
				"updateAt":    t,
			},
		},
		options.UpdateOne().SetUpsert(true),
	)
	return err
}
