package model

import (
	"context"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsJenkinsAgentCollectionName = "devops_jenkins_agent"

type JenkinsAgentContainer struct {
	Name  string `bson:"name,omitempty" json:"name"`
	Image string `bson:"image,omitempty" json:"image"`
}

type DevopsJenkinsAgent struct {
	ID         bson.ObjectID           `bson:"_id,omitempty"`
	ChannelID  string                  `bson:"channelId,omitempty"`
	Name       string                  `bson:"name,omitempty"`
	Code       string                  `bson:"code,omitempty"`
	AgentType  string                  `bson:"agentType,omitempty"`
	MatchMode  string                  `bson:"matchMode,omitempty"`
	MatchValue string                  `bson:"matchValue,omitempty"`
	Cloud      string                  `bson:"cloud,omitempty"`
	PodYaml    string                  `bson:"podYaml,omitempty"`
	Containers []JenkinsAgentContainer `bson:"containers,omitempty"`
	Status     int64                   `bson:"status"`
	CreatedBy  string                  `bson:"createdBy,omitempty"`
	UpdatedBy  string                  `bson:"updatedBy,omitempty"`
	CreateAt   time.Time               `bson:"createAt,omitempty"`
	UpdateAt   time.Time               `bson:"updateAt,omitempty"`
	IsDeleted  bool                    `bson:"isDeleted"`
}

type DevopsJenkinsAgentListFilter struct {
	ChannelID string
	Name      string
	AgentType string
	Status    int64
	Page      uint64
	PageSize  uint64
}

type DevopsJenkinsAgentModel struct {
	conn *mon.Model
}

func NewDevopsJenkinsAgentModel(url, db string) *DevopsJenkinsAgentModel {
	return &DevopsJenkinsAgentModel{
		conn: mon.MustNewModel(url, db, DevopsJenkinsAgentCollectionName),
	}
}

func DefaultJenkinsAgentContainers() []JenkinsAgentContainer {
	return []JenkinsAgentContainer{
		{Name: "jnlp"},
		{Name: "git"},
		{Name: "go"},
		{Name: "maven"},
	}
}

func NormalizeJenkinsAgentContainers(items []JenkinsAgentContainer) []JenkinsAgentContainer {
	result := make([]JenkinsAgentContainer, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, JenkinsAgentContainer{
			Name:  name,
			Image: strings.TrimSpace(item.Image),
		})
	}
	return result
}

func (m *DevopsJenkinsAgentModel) Insert(ctx context.Context, data *DevopsJenkinsAgent) error {
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsJenkinsAgentModel) FindOne(ctx context.Context, id string) (*DevopsJenkinsAgent, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsJenkinsAgent
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsJenkinsAgentModel) Update(ctx context.Context, data *DevopsJenkinsAgent) error {
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"name":       data.Name,
			"code":       data.Code,
			"agentType":  data.AgentType,
			"matchMode":  data.MatchMode,
			"matchValue": data.MatchValue,
			"cloud":      data.Cloud,
			"podYaml":    data.PodYaml,
			"containers": data.Containers,
			"status":     data.Status,
			"updatedBy":  data.UpdatedBy,
			"updateAt":   data.UpdateAt,
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

func (m *DevopsJenkinsAgentModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsJenkinsAgentModel) CountByChannel(ctx context.Context, channelID string) (uint64, error) {
	total, err := m.conn.CountDocuments(ctx, bson.M{
		"channelId": channelID,
		"isDeleted": false,
	})
	if err != nil {
		return 0, err
	}
	return uint64(total), nil
}

func (m *DevopsJenkinsAgentModel) List(ctx context.Context, filter DevopsJenkinsAgentListFilter) ([]*DevopsJenkinsAgent, uint64, error) {
	query := bson.M{"isDeleted": false}
	if filter.ChannelID != "" {
		query["channelId"] = filter.ChannelID
	}
	if filter.Name != "" {
		query["name"] = bson.M{"$regex": filter.Name, "$options": "i"}
	}
	if filter.AgentType != "" {
		if filter.AgentType == "dynamic" {
			query["agentType"] = bson.M{"$in": []string{"dynamic", "pod"}}
		} else {
			query["agentType"] = filter.AgentType
		}
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
		SetSort(bson.D{{Key: "channelId", Value: 1}, {Key: "createAt", Value: -1}}).
		SetSkip(int64((page.Page - 1) * page.PageSize)).
		SetLimit(int64(page.PageSize))

	var data []*DevopsJenkinsAgent
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}

func EnsureDefaultJenkinsAgents(ctx context.Context, channelModel *DevopsChannelModel, agentModel *DevopsJenkinsAgentModel) error {
	page := uint64(1)
	for {
		channels, total, err := channelModel.List(ctx, DevopsChannelListFilter{
			ChannelType: "jenkins",
			Status:      -1,
			Page:        page,
			PageSize:    200,
		})
		if err != nil {
			return err
		}
		for _, channel := range channels {
			if channel == nil {
				continue
			}
			channelID := channel.ID.Hex()
			count, err := agentModel.CountByChannel(ctx, channelID)
			if err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			if err := agentModel.Insert(ctx, &DevopsJenkinsAgent{
				ChannelID:  channelID,
				Name:       "主节点",
				Code:       "master",
				AgentType:  "static",
				MatchMode:  "name",
				MatchValue: "master",
				Status:     1,
				CreatedBy:  "system",
				UpdatedBy:  "system",
			}); err != nil {
				return err
			}
		}
		if page*200 >= total || len(channels) == 0 {
			return nil
		}
		page++
	}
}
