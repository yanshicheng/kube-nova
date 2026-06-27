package model

import (
	"context"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsPipelineRunCounterCollectionName = "devops_pipeline_run_counter"

type DevopsPipelineRunCounter struct {
	ID              bson.ObjectID `bson:"_id,omitempty"`
	CounterKey      string        `bson:"counterKey,omitempty"`
	EngineType      string        `bson:"engineType,omitempty"`
	TektonNamespace string        `bson:"tektonNamespace,omitempty"`
	SystemCode      string        `bson:"systemCode,omitempty"`
	PipelineCode    string        `bson:"pipelineCode,omitempty"`
	Current         int64         `bson:"current,omitempty"`
	CreateAt        time.Time     `bson:"createAt,omitempty"`
	UpdateAt        time.Time     `bson:"updateAt,omitempty"`
	IsDeleted       bool          `bson:"isDeleted"`
}

type DevopsPipelineRunCounterModel struct {
	conn *mon.Model
}

func NewDevopsPipelineRunCounterModel(url, db string) *DevopsPipelineRunCounterModel {
	return &DevopsPipelineRunCounterModel{
		conn: mon.MustNewModel(url, db, DevopsPipelineRunCounterCollectionName),
	}
}

func (m *DevopsPipelineRunCounterModel) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "counterKey", Value: 1}},
			Options: options.Index().SetName("idx_run_counter_key").SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "engineType", Value: 1},
				{Key: "tektonNamespace", Value: 1},
				{Key: "systemCode", Value: 1},
				{Key: "pipelineCode", Value: 1},
			},
			Options: options.Index().SetName("idx_run_counter_dimensions"),
		},
	})
	return err
}

func (m *DevopsPipelineRunCounterModel) NextTektonBuildID(ctx context.Context, namespace, systemCode, pipelineCode string) (int64, error) {
	namespace = strings.TrimSpace(namespace)
	systemCode = strings.TrimSpace(systemCode)
	pipelineCode = strings.TrimSpace(pipelineCode)
	key := strings.Join([]string{"tekton", namespace, systemCode, pipelineCode}, ":")
	t := now()
	var data DevopsPipelineRunCounter
	err := m.conn.FindOneAndUpdate(ctx,
		&data,
		bson.M{"counterKey": key},
		bson.M{
			"$setOnInsert": bson.M{
				"_id":             bson.NewObjectID(),
				"counterKey":      key,
				"engineType":      "tekton",
				"tektonNamespace": namespace,
				"systemCode":      systemCode,
				"pipelineCode":    pipelineCode,
				"createAt":        t,
				"isDeleted":       false,
			},
			"$set": bson.M{
				"updateAt": t,
			},
			"$inc": bson.M{"current": int64(1)},
		},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	)
	if err != nil {
		return 0, err
	}
	return data.Current, nil
}
