package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsHostCollectionName = "devops_host"

type DevopsHost struct {
	ID               bson.ObjectID `bson:"_id,omitempty"`
	Name             string        `bson:"name,omitempty"`
	IP               string        `bson:"ip,omitempty"`
	Port             int64         `bson:"port,omitempty"`
	CredentialID     string        `bson:"credentialId,omitempty"`
	Labels           string        `bson:"labels,omitempty"`
	Description      string        `bson:"description,omitempty"`
	HealthStatus     string        `bson:"healthStatus,omitempty"`
	LastCheckAt      int64         `bson:"lastCheckAt,omitempty"`
	LastCheckMessage string        `bson:"lastCheckMessage,omitempty"`
	Metadata         string        `bson:"metadata,omitempty"`
	Status           int64         `bson:"status"`
	CreatedBy        string        `bson:"createdBy,omitempty"`
	UpdatedBy        string        `bson:"updatedBy,omitempty"`
	CreateAt         time.Time     `bson:"createAt,omitempty"`
	UpdateAt         time.Time     `bson:"updateAt,omitempty"`
	IsDeleted        bool          `bson:"isDeleted"`
}

type DevopsHostListFilter struct {
	Name     string
	IP       string
	Status   int64
	Page     uint64
	PageSize uint64
}

type DevopsHostModel struct {
	conn *mon.Model
}

func NewDevopsHostModel(url, db string) *DevopsHostModel {
	return &DevopsHostModel{
		conn: mon.MustNewModel(url, db, DevopsHostCollectionName),
	}
}

func (m *DevopsHostModel) Insert(ctx context.Context, data *DevopsHost) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	if data.Port == 0 {
		data.Port = 22
	}
	if data.HealthStatus == "" {
		data.HealthStatus = "unknown"
	}
	data.IsDeleted = false
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsHostModel) FindOne(ctx context.Context, id string) (*DevopsHost, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsHost
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsHostModel) Update(ctx context.Context, data *DevopsHost) error {
	data.UpdateAt = now()
	if data.Port == 0 {
		data.Port = 22
	}
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"name":         data.Name,
			"ip":           data.IP,
			"port":         data.Port,
			"credentialId": data.CredentialID,
			"labels":       data.Labels,
			"description":  data.Description,
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

func (m *DevopsHostModel) UpdateHealth(ctx context.Context, id, status, message, metadata, updatedBy string) error {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return err
	}
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": oid, "isDeleted": false},
		bson.M{"$set": bson.M{
			"healthStatus":     status,
			"lastCheckAt":      time.Now().Unix(),
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

func (m *DevopsHostModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsHostModel) List(ctx context.Context, filter DevopsHostListFilter) ([]*DevopsHost, uint64, error) {
	query := bson.M{"isDeleted": false}
	if filter.Name != "" {
		query["name"] = bson.M{"$regex": filter.Name, "$options": "i"}
	}
	if filter.IP != "" {
		query["ip"] = bson.M{"$regex": filter.IP, "$options": "i"}
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

	var data []*DevopsHost
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}

func (m *DevopsHostModel) ListByCredential(ctx context.Context, credentialID string, limit int64) ([]*DevopsHost, uint64, error) {
	query := bson.M{"credentialId": credentialID, "isDeleted": false}
	total, err := m.conn.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(bson.D{{Key: "createAt", Value: -1}})
	if limit > 0 {
		opts.SetLimit(limit)
	}
	var data []*DevopsHost
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}
