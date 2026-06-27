package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsStepSyncCollectionName = "devops_step_sync"

type DevopsStepSync struct {
	ID                 bson.ObjectID `bson:"_id,omitempty"`
	StepID             string        `bson:"stepId,omitempty"`
	StepCode           string        `bson:"stepCode,omitempty"`
	StepName           string        `bson:"stepName,omitempty"`
	ChannelID          string        `bson:"channelId,omitempty"`
	ChannelCode        string        `bson:"channelCode,omitempty"`
	ChannelName        string        `bson:"channelName,omitempty"`
	ResourceKind       string        `bson:"resourceKind,omitempty"`
	ResourceName       string        `bson:"resourceName,omitempty"`
	ResourceNamespace  string        `bson:"resourceNamespace,omitempty"`
	ResourceAPIVersion string        `bson:"resourceApiVersion,omitempty"`
	ResourceHash       string        `bson:"resourceHash,omitempty"`
	SyncStatus         string        `bson:"syncStatus,omitempty"`
	SyncMessage        string        `bson:"syncMessage,omitempty"`
	LastSyncAt         time.Time     `bson:"lastSyncAt,omitempty"`
	CreatedBy          string        `bson:"createdBy,omitempty"`
	UpdatedBy          string        `bson:"updatedBy,omitempty"`
	CreateAt           time.Time     `bson:"createAt,omitempty"`
	UpdateAt           time.Time     `bson:"updateAt,omitempty"`
	IsDeleted          bool          `bson:"isDeleted"`
}

type DevopsStepSyncModel struct {
	conn *mon.Model
}

type DevopsStepSyncListFilter struct {
	StepID    string
	ChannelID string
	Status    string
	Page      uint64
	PageSize  uint64
}

func NewDevopsStepSyncModel(url, db string) *DevopsStepSyncModel {
	return &DevopsStepSyncModel{
		conn: mon.MustNewModel(url, db, DevopsStepSyncCollectionName),
	}
}

func (m *DevopsStepSyncModel) Upsert(ctx context.Context, data *DevopsStepSync) error {
	t := now()
	_, err := m.conn.UpdateOne(ctx,
		bson.M{
			"stepId":    data.StepID,
			"channelId": data.ChannelID,
			"isDeleted": false,
		},
		bson.M{
			"$setOnInsert": bson.M{
				"_id":       bson.NewObjectID(),
				"stepId":    data.StepID,
				"channelId": data.ChannelID,
				"createdBy": data.CreatedBy,
				"createAt":  t,
				"isDeleted": false,
			},
			"$set": bson.M{
				"stepCode":           data.StepCode,
				"stepName":           data.StepName,
				"channelCode":        data.ChannelCode,
				"channelName":        data.ChannelName,
				"resourceKind":       data.ResourceKind,
				"resourceName":       data.ResourceName,
				"resourceNamespace":  data.ResourceNamespace,
				"resourceApiVersion": data.ResourceAPIVersion,
				"resourceHash":       data.ResourceHash,
				"syncStatus":         data.SyncStatus,
				"syncMessage":        data.SyncMessage,
				"lastSyncAt":         data.LastSyncAt,
				"updatedBy":          data.UpdatedBy,
				"updateAt":           t,
			},
		},
		options.UpdateOne().SetUpsert(true),
	)
	return err
}

func (m *DevopsStepSyncModel) List(ctx context.Context, filter DevopsStepSyncListFilter) ([]*DevopsStepSync, uint64, error) {
	query := bson.M{"isDeleted": false}
	if filter.StepID != "" {
		query["stepId"] = filter.StepID
	}
	if filter.ChannelID != "" {
		query["channelId"] = filter.ChannelID
	}
	if filter.Status != "" {
		query["syncStatus"] = filter.Status
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
	var data []*DevopsStepSync
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}

func (m *DevopsStepSyncModel) MarkDeletedByStep(ctx context.Context, stepID, updatedBy string) error {
	_, err := m.conn.UpdateMany(ctx,
		bson.M{"stepId": stepID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"syncStatus":  "deleted",
			"syncMessage": "",
			"updatedBy":   updatedBy,
			"updateAt":    now(),
		}},
	)
	return err
}
