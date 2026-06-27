package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsChannelCollectionName = "devops_channel"

type DevopsChannel struct {
	ID                 bson.ObjectID `bson:"_id,omitempty"`
	GroupID            string        `bson:"groupId,omitempty"`
	Name               string        `bson:"name,omitempty"`
	Code               string        `bson:"code,omitempty"`
	ChannelType        string        `bson:"channelType,omitempty"`
	Endpoint           string        `bson:"endpoint,omitempty"`
	Description        string        `bson:"description,omitempty"`
	GlobalCredentialID string        `bson:"globalCredentialId,omitempty"`
	CredentialID       string        `bson:"credentialId,omitempty"`
	Config             string        `bson:"config,omitempty"`
	Labels             string        `bson:"labels,omitempty"`
	AuthType           string        `bson:"authType,omitempty"`
	Username           string        `bson:"username,omitempty"`
	Password           string        `bson:"password,omitempty"`
	Token              string        `bson:"token,omitempty"`
	InsecureSkipTLS    bool          `bson:"insecureSkipTls,omitempty"`
	HealthStatus       string        `bson:"healthStatus,omitempty"`
	LastCheckAt        int64         `bson:"lastCheckAt,omitempty"`
	LastCheckMessage   string        `bson:"lastCheckMessage,omitempty"`
	Metadata           string        `bson:"metadata,omitempty"`
	Icon               string        `bson:"icon,omitempty"`
	IconColor          string        `bson:"iconColor,omitempty"`
	Status             int64         `bson:"status"`
	IsSystem           bool          `bson:"isSystem,omitempty"`
	CreatedBy          string        `bson:"createdBy,omitempty"`
	UpdatedBy          string        `bson:"updatedBy,omitempty"`
	CreateAt           time.Time     `bson:"createAt,omitempty"`
	UpdateAt           time.Time     `bson:"updateAt,omitempty"`
	IsDeleted          bool          `bson:"isDeleted"`
}

func (m *DevopsChannelModel) FindOneByCode(ctx context.Context, code string) (*DevopsChannel, error) {
	var data DevopsChannel
	err := m.conn.FindOne(ctx, &data, bson.M{"code": code, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

type DevopsChannelListFilter struct {
	Name        string
	Code        string
	GroupID     string
	ChannelType string
	Status      int64
	Page        uint64
	PageSize    uint64
}

type DevopsChannelModel struct {
	conn *mon.Model
}

func NewDevopsChannelModel(url, db string) *DevopsChannelModel {
	return &DevopsChannelModel{
		conn: mon.MustNewModel(url, db, DevopsChannelCollectionName),
	}
}

func (m *DevopsChannelModel) Insert(ctx context.Context, data *DevopsChannel) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	if data.AuthType == "" {
		data.AuthType = "none"
	}
	if data.HealthStatus == "" {
		data.HealthStatus = "unknown"
	}
	data.IsDeleted = false
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsChannelModel) FindOne(ctx context.Context, id string) (*DevopsChannel, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsChannel
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsChannelModel) FindByIDs(ctx context.Context, ids []string) (map[string]*DevopsChannel, error) {
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
		return map[string]*DevopsChannel{}, nil
	}
	var data []*DevopsChannel
	if err := m.conn.Find(ctx, &data, bson.M{
		"_id":       bson.M{"$in": objectIDs},
		"isDeleted": false,
	}); err != nil {
		return nil, err
	}
	result := make(map[string]*DevopsChannel, len(data))
	for _, item := range data {
		if item == nil {
			continue
		}
		result[item.ID.Hex()] = item
	}
	return result, nil
}

func (m *DevopsChannelModel) Update(ctx context.Context, data *DevopsChannel) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"groupId":            data.GroupID,
			"name":               data.Name,
			"channelType":        data.ChannelType,
			"endpoint":           data.Endpoint,
			"description":        data.Description,
			"globalCredentialId": data.GlobalCredentialID,
			"credentialId":       data.CredentialID,
			"config":             data.Config,
			"labels":             data.Labels,
			"authType":           data.AuthType,
			"username":           data.Username,
			"password":           data.Password,
			"token":              data.Token,
			"insecureSkipTls":    data.InsecureSkipTLS,
			"icon":               data.Icon,
			"iconColor":          data.IconColor,
			"status":             data.Status,
			"updatedBy":          data.UpdatedBy,
			"updateAt":           data.UpdateAt,
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

func (m *DevopsChannelModel) MarkSystem(ctx context.Context, id string, data *DevopsChannel) error {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = m.conn.UpdateOne(ctx,
		bson.M{"_id": oid, "isDeleted": false},
		bson.M{"$set": bson.M{
			"groupId":          data.GroupID,
			"isSystem":         true,
			"authType":         data.AuthType,
			"credentialId":     data.CredentialID,
			"healthStatus":     data.HealthStatus,
			"lastCheckAt":      time.Now().Unix(),
			"lastCheckMessage": data.LastCheckMessage,
			"metadata":         data.Metadata,
			"icon":             data.Icon,
			"iconColor":        data.IconColor,
			"updatedBy":        data.UpdatedBy,
			"updateAt":         now(),
		}},
	)
	return err
}

func (m *DevopsChannelModel) UpdateHealth(ctx context.Context, id, status, message, metadata, updatedBy string) error {
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

func (m *DevopsChannelModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsChannelModel) DeleteSystemPlaceholders(ctx context.Context, updatedBy string) error {
	_, err := m.conn.UpdateMany(ctx,
		bson.M{
			"isSystem":  true,
			"isDeleted": false,
			"endpoint":  bson.M{"$regex": "^system://"},
		},
		bson.M{"$set": bson.M{"isDeleted": true, "updatedBy": updatedBy, "updateAt": now()}},
	)
	return err
}

func (m *DevopsChannelModel) CountByGroup(ctx context.Context, groupID string) (uint64, error) {
	total, err := m.conn.CountDocuments(ctx, bson.M{"groupId": groupID, "isDeleted": false})
	if err != nil {
		return 0, err
	}
	return uint64(total), nil
}

func (m *DevopsChannelModel) ReplaceChannelType(ctx context.Context, from, to, updatedBy string) error {
	_, err := m.conn.UpdateMany(ctx,
		bson.M{"channelType": from, "isDeleted": false},
		bson.M{"$set": bson.M{"channelType": to, "updatedBy": updatedBy, "updateAt": now()}},
	)
	return err
}

func (m *DevopsChannelModel) MoveChannelTypeGroup(ctx context.Context, channelType, groupID, updatedBy string) error {
	_, err := m.conn.UpdateMany(ctx,
		bson.M{"channelType": channelType, "isDeleted": false},
		bson.M{"$set": bson.M{"groupId": groupID, "updatedBy": updatedBy, "updateAt": now()}},
	)
	return err
}

func (m *DevopsChannelModel) List(ctx context.Context, filter DevopsChannelListFilter) ([]*DevopsChannel, uint64, error) {
	query := bson.M{"isDeleted": false, "isSystem": bson.M{"$ne": true}}
	if filter.Name != "" {
		query["name"] = bson.M{"$regex": filter.Name, "$options": "i"}
	}
	if filter.Code != "" {
		query["code"] = bson.M{"$regex": filter.Code, "$options": "i"}
	}
	if filter.GroupID != "" {
		query["groupId"] = filter.GroupID
	}
	if filter.ChannelType != "" {
		query["channelType"] = filter.ChannelType
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

	var data []*DevopsChannel
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}

func (m *DevopsChannelModel) ListByCredential(ctx context.Context, credentialID string, limit int64) ([]*DevopsChannel, uint64, error) {
	query := bson.M{
		"isDeleted": false,
		"$or": []bson.M{
			{"credentialId": credentialID},
			{"globalCredentialId": credentialID},
		},
	}
	total, err := m.conn.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(bson.D{{Key: "createAt", Value: -1}})
	if limit > 0 {
		opts.SetLimit(limit)
	}
	var data []*DevopsChannel
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}

func (m *DevopsChannelModel) ListAll(ctx context.Context) ([]*DevopsChannel, error) {
	var data []*DevopsChannel
	opts := options.Find().SetSort(bson.D{{Key: "createAt", Value: -1}})
	if err := m.conn.Find(ctx, &data, bson.M{"isDeleted": false}, opts); err != nil {
		return nil, err
	}
	return data, nil
}
