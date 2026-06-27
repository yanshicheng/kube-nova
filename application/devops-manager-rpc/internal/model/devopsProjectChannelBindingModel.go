package model

import (
	"context"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsProjectChannelBindingCollectionName = "devops_project_channel_binding"

type DevopsProjectChannelBinding struct {
	ID                       bson.ObjectID `bson:"_id,omitempty"`
	ProjectID                string        `bson:"projectId,omitempty"`
	ChannelID                string        `bson:"channelId,omitempty"`
	ChannelGroupCode         string        `bson:"channelGroupCode,omitempty"`
	ChannelName              string        `bson:"channelName,omitempty"`
	ChannelCode              string        `bson:"channelCode,omitempty"`
	ChannelType              string        `bson:"channelType,omitempty"`
	UsageScope               string        `bson:"usageScope,omitempty"`
	IsDefault                bool          `bson:"isDefault,omitempty"`
	AllowUseGlobalCredential bool          `bson:"allowUseGlobalCredential,omitempty"`
	ProjectCredentialID      string        `bson:"projectCredentialId,omitempty"`
	BindingConfig            string        `bson:"bindingConfig,omitempty"`
	HealthStatus             string        `bson:"healthStatus,omitempty"`
	LastCheckAt              int64         `bson:"lastCheckAt,omitempty"`
	LastCheckMessage         string        `bson:"lastCheckMessage,omitempty"`
	Metadata                 string        `bson:"metadata,omitempty"`
	Status                   int64         `bson:"status"`
	CreatedBy                string        `bson:"createdBy,omitempty"`
	UpdatedBy                string        `bson:"updatedBy,omitempty"`
	CreateAt                 time.Time     `bson:"createAt,omitempty"`
	UpdateAt                 time.Time     `bson:"updateAt,omitempty"`
	IsDeleted                bool          `bson:"isDeleted"`
}

type DevopsProjectChannelBindingListFilter struct {
	ProjectID        string
	ProjectIDs       []string
	Restricted       bool
	ChannelID        string
	ChannelGroupCode string
	ChannelType      string
	UsageScope       string
	Status           int64
	Page             uint64
	PageSize         uint64
}

type DevopsProjectChannelBindingModel struct {
	conn *mon.Model
}

func NewDevopsProjectChannelBindingModel(url, db string) *DevopsProjectChannelBindingModel {
	return &DevopsProjectChannelBindingModel{
		conn: mon.MustNewModel(url, db, DevopsProjectChannelBindingCollectionName),
	}
}

func (m *DevopsProjectChannelBindingModel) Insert(ctx context.Context, data *DevopsProjectChannelBinding) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	if data.HealthStatus == "" {
		data.HealthStatus = "unknown"
	}
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsProjectChannelBindingModel) FindOne(ctx context.Context, id string) (*DevopsProjectChannelBinding, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsProjectChannelBinding
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsProjectChannelBindingModel) FindActiveByProjectChannel(ctx context.Context, projectID, channelID string) (*DevopsProjectChannelBinding, error) {
	var data DevopsProjectChannelBinding
	err := m.conn.FindOne(ctx, &data, bson.M{
		"projectId": projectID,
		"channelId": channelID,
		"status":    1,
		"isDeleted": false,
	})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsProjectChannelBindingModel) HasDefaultActiveByProjectGroup(ctx context.Context, projectID, groupCode string) (bool, error) {
	scope := groupCode
	if groupCode == BuildChannelGroupCode {
		scope = BuildUsageScope
	}
	query := bson.M{
		"projectId": projectID,
		"isDefault": true,
		"status":    1,
		"isDeleted": false,
		"$or": []bson.M{
			{"channelGroupCode": groupCode},
			{"usageScope": scope},
		},
	}
	var data DevopsProjectChannelBinding
	err := m.conn.FindOne(ctx, &data, query)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (m *DevopsProjectChannelBindingModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsProjectChannelBindingModel) UpdateBinding(ctx context.Context, data *DevopsProjectChannelBinding) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"isDefault":                data.IsDefault,
			"allowUseGlobalCredential": data.AllowUseGlobalCredential,
			"projectCredentialId":      data.ProjectCredentialID,
			"bindingConfig":            data.BindingConfig,
			"status":                   data.Status,
			"updatedBy":                data.UpdatedBy,
			"updateAt":                 data.UpdateAt,
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

func (m *DevopsProjectChannelBindingModel) UpdateHealth(ctx context.Context, id, status, message, metadata, updatedBy string, checkedAt int64) error {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return err
	}
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": oid, "isDeleted": false},
		bson.M{"$set": bson.M{
			"healthStatus":     status,
			"lastCheckAt":      checkedAt,
			"lastCheckMessage": message,
			"metadata":         metadata,
			"updatedBy":        updatedBy,
			"updateAt":         now(),
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

func (m *DevopsProjectChannelBindingModel) ClearDefaultByProjectGroup(ctx context.Context, projectID, groupCode, exceptID, updatedBy string) error {
	query := bson.M{
		"projectId":        projectID,
		"isDefault":        true,
		"isDeleted":        false,
		"channelGroupCode": groupCode,
	}
	if exceptID != "" {
		oid, err := objectIDFromHex(exceptID)
		if err != nil {
			return err
		}
		query["_id"] = bson.M{"$ne": oid}
	}
	_, err := m.conn.UpdateMany(ctx,
		query,
		bson.M{"$set": bson.M{"isDefault": false, "updatedBy": updatedBy, "updateAt": now()}},
	)
	return err
}

func (m *DevopsProjectChannelBindingModel) DeleteSoftByProjectScope(ctx context.Context, projectID, usageScope, updatedBy string) error {
	_, err := m.conn.UpdateMany(ctx,
		bson.M{"projectId": projectID, "usageScope": usageScope, "isDeleted": false},
		bson.M{"$set": bson.M{"isDeleted": true, "updatedBy": updatedBy, "updateAt": now()}},
	)
	return err
}

func (m *DevopsProjectChannelBindingModel) DeleteSoftByProjectGroup(ctx context.Context, projectID, groupCode, updatedBy string) error {
	scope := groupCode
	if groupCode == BuildChannelGroupCode {
		scope = BuildUsageScope
	}
	_, err := m.conn.UpdateMany(ctx,
		bson.M{
			"projectId": projectID,
			"isDeleted": false,
			"$or": []bson.M{
				{"channelGroupCode": groupCode},
				{"usageScope": scope},
			},
		},
		bson.M{"$set": bson.M{"isDeleted": true, "updatedBy": updatedBy, "updateAt": now()}},
	)
	return err
}

func (m *DevopsProjectChannelBindingModel) ReplaceChannelType(ctx context.Context, from, to, updatedBy string) error {
	_, err := m.conn.UpdateMany(ctx,
		bson.M{"channelType": from, "isDeleted": false},
		bson.M{"$set": bson.M{"channelType": to, "updatedBy": updatedBy, "updateAt": now()}},
	)
	return err
}

func (m *DevopsProjectChannelBindingModel) MoveChannelTypeGroup(ctx context.Context, channelType, groupCode, updatedBy string) error {
	_, err := m.conn.UpdateMany(ctx,
		bson.M{"channelType": channelType, "isDeleted": false},
		bson.M{"$set": bson.M{
			"channelGroupCode": groupCode,
			"usageScope":       groupCode,
			"isDefault":        false,
			"updatedBy":        updatedBy,
			"updateAt":         now(),
		}},
	)
	return err
}

func (m *DevopsProjectChannelBindingModel) List(ctx context.Context, filter DevopsProjectChannelBindingListFilter) ([]*DevopsProjectChannelBinding, uint64, error) {
	query := bson.M{"isDeleted": false}
	if filter.ProjectID != "" {
		if filter.Restricted && !stringSliceContains(filter.ProjectIDs, filter.ProjectID) {
			return []*DevopsProjectChannelBinding{}, 0, nil
		}
		query["projectId"] = filter.ProjectID
	} else if filter.Restricted {
		if len(filter.ProjectIDs) == 0 {
			return []*DevopsProjectChannelBinding{}, 0, nil
		}
		query["projectId"] = bson.M{"$in": filter.ProjectIDs}
	}
	if filter.ChannelID != "" {
		query["channelId"] = filter.ChannelID
	}
	if filter.ChannelGroupCode != "" {
		scope := filter.ChannelGroupCode
		if filter.ChannelGroupCode == BuildChannelGroupCode {
			scope = BuildUsageScope
		}
		query["$or"] = []bson.M{
			{"channelGroupCode": filter.ChannelGroupCode},
			{"usageScope": scope},
		}
	}
	if filter.ChannelType != "" {
		query["channelType"] = filter.ChannelType
	}
	if filter.UsageScope != "" {
		query["usageScope"] = filter.UsageScope
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
		SetSort(bson.D{{Key: "projectId", Value: 1}, {Key: "isDefault", Value: -1}, {Key: "createAt", Value: -1}}).
		SetSkip(int64((page.Page - 1) * page.PageSize)).
		SetLimit(int64(page.PageSize))

	var data []*DevopsProjectChannelBinding
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}

func (m *DevopsProjectChannelBindingModel) ListByCredential(ctx context.Context, credentialID string, limit int64) ([]*DevopsProjectChannelBinding, uint64, error) {
	query := bson.M{"projectCredentialId": credentialID, "isDeleted": false}
	total, err := m.conn.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(bson.D{{Key: "createAt", Value: -1}})
	if limit > 0 {
		opts.SetLimit(limit)
	}
	var data []*DevopsProjectChannelBinding
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}

func stringSliceContains(data []string, target string) bool {
	for _, item := range data {
		if item == target {
			return true
		}
	}
	return false
}
