package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsCredentialSyncCollectionName = "devops_credential_sync"

type DevopsCredentialSync struct {
	ID                    bson.ObjectID `bson:"_id,omitempty"`
	ProjectID             string        `bson:"projectId,omitempty"`
	BuildChannelBindingID string        `bson:"buildChannelBindingId,omitempty"`
	CredentialID          string        `bson:"credentialId,omitempty"`
	JenkinsCredentialID   string        `bson:"jenkinsCredentialId,omitempty"`
	SyncStatus            string        `bson:"syncStatus,omitempty"`
	SyncMessage           string        `bson:"syncMessage,omitempty"`
	LastSyncAt            time.Time     `bson:"lastSyncAt,omitempty"`
	CreatedBy             string        `bson:"createdBy,omitempty"`
	UpdatedBy             string        `bson:"updatedBy,omitempty"`
	CreateAt              time.Time     `bson:"createAt,omitempty"`
	UpdateAt              time.Time     `bson:"updateAt,omitempty"`
	IsDeleted             bool          `bson:"isDeleted"`
}

type DevopsCredentialSyncModel struct {
	conn *mon.Model
}

type DevopsCredentialSyncListFilter struct {
	ProjectID             string
	BuildChannelBindingID string
	Page                  uint64
	PageSize              uint64
}

func NewDevopsCredentialSyncModel(url, db string) *DevopsCredentialSyncModel {
	return &DevopsCredentialSyncModel{
		conn: mon.MustNewModel(url, db, DevopsCredentialSyncCollectionName),
	}
}

func (m *DevopsCredentialSyncModel) Upsert(ctx context.Context, data *DevopsCredentialSync) error {
	t := now()
	setOnInsert := bson.M{
		"_id":                   bson.NewObjectID(),
		"projectId":             data.ProjectID,
		"credentialId":          data.CredentialID,
		"buildChannelBindingId": data.BuildChannelBindingID,
		"createdBy":             data.CreatedBy,
		"createAt":              t,
		"isDeleted":             false,
	}
	_, err := m.conn.UpdateOne(ctx,
		bson.M{
			"projectId":             data.ProjectID,
			"credentialId":          data.CredentialID,
			"buildChannelBindingId": data.BuildChannelBindingID,
			"isDeleted":             false,
		},
		bson.M{
			"$setOnInsert": setOnInsert,
			"$set": bson.M{
				"jenkinsCredentialId": data.JenkinsCredentialID,
				"syncStatus":          data.SyncStatus,
				"syncMessage":         data.SyncMessage,
				"lastSyncAt":          data.LastSyncAt,
				"updatedBy":           data.UpdatedBy,
				"updateAt":            t,
			},
		},
		options.UpdateOne().SetUpsert(true),
	)
	return err
}

func (m *DevopsCredentialSyncModel) FindOne(ctx context.Context, projectID, buildChannelBindingID, credentialID string) (*DevopsCredentialSync, error) {
	var data DevopsCredentialSync
	err := m.conn.FindOne(ctx, &data, bson.M{
		"projectId":             projectID,
		"buildChannelBindingId": buildChannelBindingID,
		"credentialId":          credentialID,
		"isDeleted":             false,
	})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsCredentialSyncModel) List(ctx context.Context, filter DevopsCredentialSyncListFilter) ([]*DevopsCredentialSync, uint64, error) {
	query := bson.M{"isDeleted": false}
	if filter.ProjectID != "" {
		query["projectId"] = filter.ProjectID
	}
	if filter.BuildChannelBindingID != "" {
		query["buildChannelBindingId"] = filter.BuildChannelBindingID
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
	var data []*DevopsCredentialSync
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}

func (m *DevopsCredentialSyncModel) MarkPendingByCredential(ctx context.Context, credentialID, message, updatedBy string) error {
	_, err := m.conn.UpdateMany(ctx,
		bson.M{"credentialId": credentialID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"syncStatus":  "pending",
			"syncMessage": message,
			"updatedBy":   updatedBy,
			"updateAt":    now(),
		}},
	)
	return err
}

func (m *DevopsCredentialSyncModel) MarkDeleted(ctx context.Context, projectID, buildChannelBindingID, credentialID, message, updatedBy string) error {
	res, err := m.conn.UpdateOne(ctx,
		bson.M{
			"projectId":             projectID,
			"buildChannelBindingId": buildChannelBindingID,
			"credentialId":          credentialID,
			"isDeleted":             false,
		},
		bson.M{"$set": bson.M{
			"syncStatus":  "deleted",
			"syncMessage": message,
			"updatedBy":   updatedBy,
			"updateAt":    now(),
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

func (m *DevopsCredentialSyncModel) LatestByCredentialIDs(ctx context.Context, credentialIDs []string) (map[string]*DevopsCredentialSync, error) {
	return m.latestByCredentialIDs(ctx, credentialIDs, "")
}

func (m *DevopsCredentialSyncModel) LatestByCredentialIDsAndBinding(ctx context.Context, credentialIDs []string, buildChannelBindingID string) (map[string]*DevopsCredentialSync, error) {
	return m.latestByCredentialIDs(ctx, credentialIDs, buildChannelBindingID)
}

func (m *DevopsCredentialSyncModel) latestByCredentialIDs(ctx context.Context, credentialIDs []string, buildChannelBindingID string) (map[string]*DevopsCredentialSync, error) {
	if len(credentialIDs) == 0 {
		return map[string]*DevopsCredentialSync{}, nil
	}
	ids := make([]string, 0, len(credentialIDs))
	seen := make(map[string]struct{}, len(credentialIDs))
	for _, id := range credentialIDs {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return map[string]*DevopsCredentialSync{}, nil
	}
	query := bson.M{
		"credentialId": bson.M{"$in": ids},
		"isDeleted":    false,
	}
	if buildChannelBindingID != "" {
		query["buildChannelBindingId"] = buildChannelBindingID
	}
	var data []*DevopsCredentialSync
	if err := m.conn.Find(ctx, &data, query, options.Find().SetSort(bson.D{{Key: "createAt", Value: -1}})); err != nil {
		return nil, err
	}
	result := make(map[string]*DevopsCredentialSync, len(ids))
	for _, item := range data {
		if item == nil {
			continue
		}
		if _, ok := result[item.CredentialID]; ok {
			continue
		}
		result[item.CredentialID] = item
	}
	return result, nil
}
