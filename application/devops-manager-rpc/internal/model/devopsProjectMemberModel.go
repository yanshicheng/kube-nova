package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsProjectMemberCollectionName = "devops_project_member"

type DevopsProjectMember struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	ProjectID string        `bson:"projectId,omitempty"`
	UserID    uint64        `bson:"userId,omitempty"`
	Username  string        `bson:"username,omitempty"`
	Nickname  string        `bson:"nickname,omitempty"`
	Role      string        `bson:"role,omitempty"`
	Status    int64         `bson:"status"`
	CreatedBy string        `bson:"createdBy,omitempty"`
	UpdatedBy string        `bson:"updatedBy,omitempty"`
	CreateAt  time.Time     `bson:"createAt,omitempty"`
	UpdateAt  time.Time     `bson:"updateAt,omitempty"`
	IsDeleted bool          `bson:"isDeleted"`
}

type DevopsProjectMemberListFilter struct {
	ProjectID string
	UserID    uint64
	Role      string
	Status    int64
	Page      uint64
	PageSize  uint64
}

type DevopsProjectMemberModel struct {
	conn *mon.Model
}

func NewDevopsProjectMemberModel(url, db string) *DevopsProjectMemberModel {
	return &DevopsProjectMemberModel{
		conn: mon.MustNewModel(url, db, DevopsProjectMemberCollectionName),
	}
}

func (m *DevopsProjectMemberModel) Insert(ctx context.Context, data *DevopsProjectMember) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsProjectMemberModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsProjectMemberModel) DeleteSoftByProject(ctx context.Context, projectID, updatedBy string) error {
	_, err := m.conn.UpdateMany(ctx,
		bson.M{"projectId": projectID, "isDeleted": false},
		bson.M{"$set": bson.M{"isDeleted": true, "updatedBy": updatedBy, "updateAt": now()}},
	)
	return err
}

func (m *DevopsProjectMemberModel) List(ctx context.Context, filter DevopsProjectMemberListFilter) ([]*DevopsProjectMember, uint64, error) {
	query := projectMemberQuery(filter)
	total, err := m.conn.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	page := normalizePage(filter.Page, filter.PageSize)
	opts := options.Find().
		SetSort(bson.D{{Key: "projectId", Value: 1}, {Key: "role", Value: 1}, {Key: "createAt", Value: -1}}).
		SetSkip(int64((page.Page - 1) * page.PageSize)).
		SetLimit(int64(page.PageSize))

	var data []*DevopsProjectMember
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}

func (m *DevopsProjectMemberModel) ListProjectIDsByUser(ctx context.Context, userID uint64) ([]string, error) {
	if userID == 0 {
		return []string{}, nil
	}
	var data []*DevopsProjectMember
	if err := m.conn.Find(ctx, &data, bson.M{
		"userId":    userID,
		"status":    int64(1),
		"isDeleted": false,
	}); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(data))
	ids := make([]string, 0, len(data))
	for _, item := range data {
		if item.ProjectID == "" {
			continue
		}
		if _, ok := seen[item.ProjectID]; ok {
			continue
		}
		seen[item.ProjectID] = struct{}{}
		ids = append(ids, item.ProjectID)
	}
	return ids, nil
}

func projectMemberQuery(filter DevopsProjectMemberListFilter) bson.M {
	query := bson.M{"isDeleted": false}
	if filter.ProjectID != "" {
		query["projectId"] = filter.ProjectID
	}
	if filter.UserID > 0 {
		query["userId"] = filter.UserID
	}
	if filter.Role != "" {
		query["role"] = filter.Role
	}
	if filter.Status >= 0 {
		query["status"] = filter.Status
	}
	return query
}
