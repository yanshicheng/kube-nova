package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsChannelGroupCollectionName = "devops_channel_group"

type DevopsChannelGroup struct {
	ID                  bson.ObjectID `bson:"_id,omitempty"`
	Name                string        `bson:"name,omitempty"`
	Code                string        `bson:"code,omitempty"`
	Description         string        `bson:"description,omitempty"`
	SortOrder           int64         `bson:"sortOrder,omitempty"`
	IsSystem            bool          `bson:"isSystem,omitempty"`
	Status              int64         `bson:"status"`
	GroupType           string        `bson:"groupType,omitempty"`
	AllowedChannelTypes []string      `bson:"allowedChannelTypes,omitempty"`
	Icon                string        `bson:"icon,omitempty"`
	IconColor           string        `bson:"iconColor,omitempty"`
	CreatedBy           string        `bson:"createdBy,omitempty"`
	UpdatedBy           string        `bson:"updatedBy,omitempty"`
	CreateAt            time.Time     `bson:"createAt,omitempty"`
	UpdateAt            time.Time     `bson:"updateAt,omitempty"`
	IsDeleted           bool          `bson:"isDeleted"`
}

// IsBuildGroup 判断分组是否为构建渠道分组。
func (g *DevopsChannelGroup) IsBuildGroup() bool {
	if g == nil {
		return false
	}
	return g.Code == BuildChannelGroupCode || g.GroupType == BuildChannelGroupCode
}

type DevopsChannelGroupListFilter struct {
	Name      string
	Code      string
	Status    int64
	GroupType string
	Page      uint64
	PageSize  uint64
}

type DevopsChannelGroupModel struct {
	conn *mon.Model
}

func NewDevopsChannelGroupModel(url, db string) *DevopsChannelGroupModel {
	return &DevopsChannelGroupModel{
		conn: mon.MustNewModel(url, db, DevopsChannelGroupCollectionName),
	}
}

func (m *DevopsChannelGroupModel) Insert(ctx context.Context, data *DevopsChannelGroup) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsChannelGroupModel) FindOne(ctx context.Context, id string) (*DevopsChannelGroup, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsChannelGroup
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsChannelGroupModel) FindOneByCode(ctx context.Context, code string) (*DevopsChannelGroup, error) {
	var data DevopsChannelGroup
	err := m.conn.FindOne(ctx, &data, bson.M{"code": code, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsChannelGroupModel) FindByIDs(ctx context.Context, ids []string) (map[string]*DevopsChannelGroup, error) {
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
		return map[string]*DevopsChannelGroup{}, nil
	}
	var data []*DevopsChannelGroup
	if err := m.conn.Find(ctx, &data, bson.M{
		"_id":       bson.M{"$in": objectIDs},
		"isDeleted": false,
	}); err != nil {
		return nil, err
	}
	result := make(map[string]*DevopsChannelGroup, len(data))
	for _, item := range data {
		if item == nil {
			continue
		}
		result[item.ID.Hex()] = item
	}
	return result, nil
}

func (m *DevopsChannelGroupModel) CountByCode(ctx context.Context, code string) (int64, error) {
	return m.conn.CountDocuments(ctx, bson.M{"code": code, "isDeleted": false})
}

func (m *DevopsChannelGroupModel) MarkSystem(ctx context.Context, id string, data *DevopsChannelGroup) error {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = m.conn.UpdateOne(ctx,
		bson.M{"_id": oid, "isDeleted": false},
		bson.M{"$set": bson.M{
			"name":                data.Name,
			"description":         data.Description,
			"sortOrder":           data.SortOrder,
			"isSystem":            true,
			"status":              data.Status,
			"groupType":           data.GroupType,
			"allowedChannelTypes": data.AllowedChannelTypes,
			"icon":                data.Icon,
			"iconColor":           data.IconColor,
			"updatedBy":           data.UpdatedBy,
			"updateAt":            now(),
		}},
	)
	return err
}

func (m *DevopsChannelGroupModel) Update(ctx context.Context, data *DevopsChannelGroup) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"name":                data.Name,
			"description":         data.Description,
			"sortOrder":           data.SortOrder,
			"status":              data.Status,
			"groupType":           data.GroupType,
			"allowedChannelTypes": data.AllowedChannelTypes,
			"icon":                data.Icon,
			"iconColor":           data.IconColor,
			"updatedBy":           data.UpdatedBy,
			"updateAt":            data.UpdateAt,
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

func (m *DevopsChannelGroupModel) AddAllowedChannelType(ctx context.Context, groupCode, channelType, updatedBy string) error {
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"code": groupCode, "isDeleted": false},
		bson.M{
			"$addToSet": bson.M{"allowedChannelTypes": channelType},
			"$set":      bson.M{"updatedBy": updatedBy, "updateAt": now()},
		},
	)
	if err != nil {
		return err
	}
	if isNotFoundUpdate(res) {
		return ErrNotFound
	}
	return nil
}

func (m *DevopsChannelGroupModel) RemoveAllowedChannelType(ctx context.Context, groupCode, channelType, updatedBy string) error {
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"code": groupCode, "isDeleted": false},
		bson.M{
			"$pull": bson.M{"allowedChannelTypes": channelType},
			"$set":  bson.M{"updatedBy": updatedBy, "updateAt": now()},
		},
	)
	if err != nil {
		return err
	}
	if isNotFoundUpdate(res) {
		return ErrNotFound
	}
	return nil
}

func (m *DevopsChannelGroupModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsChannelGroupModel) List(ctx context.Context, filter DevopsChannelGroupListFilter) ([]*DevopsChannelGroup, uint64, error) {
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
	if filter.GroupType != "" {
		query["groupType"] = filter.GroupType
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

	var data []*DevopsChannelGroup
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}
