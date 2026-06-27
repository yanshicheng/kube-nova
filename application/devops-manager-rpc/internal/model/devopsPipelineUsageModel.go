package model

import (
	"context"
	"regexp"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsPipelineCollectionName = "devops_pipeline"

type DevopsPipelineUsage struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	ProjectID string        `bson:"projectId,omitempty"`
	Name      string        `bson:"name,omitempty"`
	Code      string        `bson:"code,omitempty"`
}

type UsageStepParamConfig struct {
	CredentialID              string `bson:"credentialId,omitempty"`
	CredentialMode            string `bson:"credentialMode,omitempty"`
	MappingField              string `bson:"mappingField,omitempty"`
	CredentialSourceParamCode string `bson:"credentialSourceParamCode,omitempty"`
	ChannelParamCode          string `bson:"channelParamCode,omitempty"`
	ChannelBindingID          string `bson:"channelBindingId,omitempty"`
}

type UsagePipelineParam struct {
	Name         string               `bson:"name,omitempty"`
	Code         string               `bson:"code,omitempty"`
	DefaultValue string               `bson:"defaultValue,omitempty"`
	CurrentValue string               `bson:"currentValue,omitempty"`
	ParamType    string               `bson:"paramType,omitempty"`
	Mode         string               `bson:"mode,omitempty"`
	Config       UsageStepParamConfig `bson:"config,omitempty"`
}

type DevopsPipelineCredentialReference struct {
	ID                    bson.ObjectID        `bson:"_id,omitempty"`
	ProjectID             string               `bson:"projectId,omitempty"`
	Name                  string               `bson:"name,omitempty"`
	Code                  string               `bson:"code,omitempty"`
	BuildChannelBindingID string               `bson:"buildChannelBindingId,omitempty"`
	Params                []UsagePipelineParam `bson:"params,omitempty"`
	UpdateAt              time.Time            `bson:"updateAt,omitempty"`
}

type DevopsPipelineUsageModel struct {
	conn *mon.Model
}

func NewDevopsPipelineUsageModel(url, db string) *DevopsPipelineUsageModel {
	return &DevopsPipelineUsageModel{
		conn: mon.MustNewModel(url, db, DevopsPipelineCollectionName),
	}
}

func (m *DevopsPipelineUsageModel) ListByCredential(ctx context.Context, credentialID string, limit int64) ([]*DevopsPipelineUsage, uint64, error) {
	credentialPattern := regexp.QuoteMeta(credentialID)
	query := bson.M{
		"isDeleted": false,
		"$or": []bson.M{
			{"params.currentValue": credentialID},
			{"params.defaultValue": credentialID},
			{"params.config.credentialId": credentialID},
			{"params.currentValue": bson.M{"$regex": credentialPattern}},
			{"params.defaultValue": bson.M{"$regex": credentialPattern}},
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
	var data []*DevopsPipelineUsage
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}

func (m *DevopsPipelineUsageModel) ListByProjectBuildChannel(ctx context.Context, projectID, buildChannelBindingID string, limit int64) ([]*DevopsPipelineCredentialReference, error) {
	query := bson.M{
		"projectId":             projectID,
		"buildChannelBindingId": buildChannelBindingID,
		"isDeleted":             false,
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "createAt", Value: -1}}).
		SetProjection(bson.M{
			"projectId":             1,
			"name":                  1,
			"code":                  1,
			"buildChannelBindingId": 1,
			"params":                1,
			"updateAt":              1,
		})
	if limit > 0 {
		opts.SetLimit(limit)
	}
	var data []*DevopsPipelineCredentialReference
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, err
	}
	return data, nil
}
