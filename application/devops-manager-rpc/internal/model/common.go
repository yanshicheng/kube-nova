package model

import (
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var (
	ErrNotFound        = mon.ErrNotFound
	ErrInvalidObjectID = errors.New("invalid objectId")
)

type PageOption struct {
	Page     uint64
	PageSize uint64
}

func normalizePage(page, pageSize uint64) PageOption {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return PageOption{Page: page, PageSize: pageSize}
}

func objectIDFromHex(id string) (bson.ObjectID, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return bson.NilObjectID, ErrInvalidObjectID
	}
	return oid, nil
}

func ObjectIDForUpdate(id string) (bson.ObjectID, error) {
	return objectIDFromHex(id)
}

func now() time.Time {
	return time.Now()
}

func isNotFoundUpdate(res *mongo.UpdateResult) bool {
	return res == nil || res.MatchedCount == 0
}

func isDuplicateKey(err error) bool {
	return mongo.IsDuplicateKeyError(err)
}

func IsDuplicateKey(err error) bool {
	return isDuplicateKey(err)
}
