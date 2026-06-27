package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/mon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const DevopsCredentialCollectionName = "devops_credential"

type DevopsCredential struct {
	ID               bson.ObjectID `bson:"_id,omitempty"`
	Name             string        `bson:"name,omitempty"`
	Code             string        `bson:"code,omitempty"`
	CredentialType   string        `bson:"credentialType,omitempty"`
	Username         string        `bson:"username,omitempty"`
	Password         string        `bson:"password,omitempty"`
	Token            string        `bson:"token,omitempty"`
	PrivateKey       string        `bson:"privateKey,omitempty"`
	Passphrase       string        `bson:"passphrase,omitempty"`
	Kubeconfig       string        `bson:"kubeconfig,omitempty"`
	SecretText       string        `bson:"secretText,omitempty"`
	Certificate      string        `bson:"certificate,omitempty"`
	JsonData         string        `bson:"jsonData,omitempty"`
	Description      string        `bson:"description,omitempty"`
	Status           int64         `bson:"status"`
	IsSystem         bool          `bson:"isSystem,omitempty"`
	Scope            string        `bson:"scope,omitempty"`
	ProjectID        string        `bson:"projectId,omitempty"`
	ChannelGroupCode string        `bson:"channelGroupCode,omitempty"`
	ChannelType      string        `bson:"channelType,omitempty"`
	CreatedBy        string        `bson:"createdBy,omitempty"`
	UpdatedBy        string        `bson:"updatedBy,omitempty"`
	CreateAt         time.Time     `bson:"createAt,omitempty"`
	UpdateAt         time.Time     `bson:"updateAt,omitempty"`
	IsDeleted        bool          `bson:"isDeleted"`
}

type CredentialSecret struct {
	Username    string
	Password    string
	Token       string
	PrivateKey  string
	Passphrase  string
	Kubeconfig  string
	SecretText  string
	Certificate string
	JsonData    string
}

type DevopsCredentialListFilter struct {
	Name             string
	Code             string
	CredentialType   string
	Status           int64
	Scope            string
	ProjectID        string
	ProjectIDs       []string
	ChannelGroupCode string
	ChannelType      string
	Restricted       bool
	Page             uint64
	PageSize         uint64
}

type DevopsCredentialModel struct {
	conn *mon.Model
}

func NewDevopsCredentialModel(url, db string) *DevopsCredentialModel {
	return &DevopsCredentialModel{
		conn: mon.MustNewModel(url, db, DevopsCredentialCollectionName),
	}
}

func (m *DevopsCredentialModel) Insert(ctx context.Context, data *DevopsCredential) error {
	if err := encryptCredential(data); err != nil {
		return err
	}
	t := now()
	data.ID = bson.NewObjectID()
	data.CreateAt = t
	data.UpdateAt = t
	data.IsDeleted = false
	_, err := m.conn.InsertOne(ctx, data)
	return err
}

func (m *DevopsCredentialModel) FindOne(ctx context.Context, id string) (*DevopsCredential, error) {
	oid, err := objectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var data DevopsCredential
	err = m.conn.FindOne(ctx, &data, bson.M{"_id": oid, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsCredentialModel) FindOneByCode(ctx context.Context, code string) (*DevopsCredential, error) {
	var data DevopsCredential
	err := m.conn.FindOne(ctx, &data, bson.M{"code": code, "isDeleted": false})
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *DevopsCredentialModel) Update(ctx context.Context, data *DevopsCredential) error {
	if err := encryptCredential(data); err != nil {
		return err
	}
	data.UpdateAt = now()
	res, err := m.conn.UpdateOne(ctx,
		bson.M{"_id": data.ID, "isDeleted": false},
		bson.M{"$set": bson.M{
			"name":             data.Name,
			"credentialType":   data.CredentialType,
			"username":         data.Username,
			"password":         data.Password,
			"token":            data.Token,
			"privateKey":       data.PrivateKey,
			"passphrase":       data.Passphrase,
			"kubeconfig":       data.Kubeconfig,
			"secretText":       data.SecretText,
			"certificate":      data.Certificate,
			"jsonData":         data.JsonData,
			"description":      data.Description,
			"status":           data.Status,
			"isSystem":         data.IsSystem,
			"scope":            data.Scope,
			"projectId":        data.ProjectID,
			"channelGroupCode": data.ChannelGroupCode,
			"channelType":      data.ChannelType,
			"updatedBy":        data.UpdatedBy,
			"updateAt":         data.UpdateAt,
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

func (m *DevopsCredentialModel) DeleteSoft(ctx context.Context, id, updatedBy string) error {
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

func (m *DevopsCredentialModel) List(ctx context.Context, filter DevopsCredentialListFilter) ([]*DevopsCredential, uint64, error) {
	query := bson.M{"isDeleted": false}
	if filter.Name != "" {
		query["name"] = bson.M{"$regex": filter.Name, "$options": "i"}
	}
	if filter.Code != "" {
		query["code"] = bson.M{"$regex": filter.Code, "$options": "i"}
	}
	if filter.CredentialType != "" {
		query["credentialType"] = filter.CredentialType
	}
	if filter.Status >= 0 {
		query["status"] = filter.Status
	}
	if filter.ChannelGroupCode != "" {
		query["channelGroupCode"] = filter.ChannelGroupCode
	}
	if filter.ChannelType != "" {
		query["channelType"] = filter.ChannelType
	}
	if filter.Restricted {
		if filter.Scope == "system" {
			return []*DevopsCredential{}, 0, nil
		}
		if filter.ProjectID == "" {
			return []*DevopsCredential{}, 0, nil
		}
		if !stringSliceContains(filter.ProjectIDs, filter.ProjectID) {
			return []*DevopsCredential{}, 0, nil
		}
		projectQuery := credentialProjectScopeQuery(nil, filter.ProjectID)
		query["$and"] = []bson.M{projectQuery}
	} else {
		if filter.Scope != "" {
			if filter.Scope == "system" {
				query["$and"] = []bson.M{credentialSystemScopeQuery()}
			} else {
				query["$and"] = []bson.M{credentialProjectScopeQuery(nil, "")}
			}
		}
		if filter.ProjectID != "" {
			query["projectId"] = filter.ProjectID
		}
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

	var data []*DevopsCredential
	if err := m.conn.Find(ctx, &data, query, opts); err != nil {
		return nil, 0, err
	}
	return data, uint64(total), nil
}

func credentialSystemScopeQuery() bson.M {
	return bson.M{"$or": []bson.M{
		{"scope": "system"},
		{"isSystem": true},
		{"scope": "", "projectId": ""},
		{"scope": bson.M{"$exists": false}, "projectId": ""},
		{"scope": bson.M{"$exists": false}, "projectId": bson.M{"$exists": false}},
	}}
}

func credentialProjectScopeQuery(projectIDs []string, projectID string) bson.M {
	if projectID != "" {
		return bson.M{
			"projectId": projectID,
			"$or": []bson.M{
				{"scope": "project"},
				{"scope": "", "isSystem": bson.M{"$ne": true}},
				{"scope": bson.M{"$exists": false}, "isSystem": bson.M{"$ne": true}},
			},
		}
	}
	if len(projectIDs) == 0 {
		return bson.M{
			"projectId": bson.M{"$nin": []string{""}},
			"$or": []bson.M{
				{"scope": "project"},
				{"scope": "", "isSystem": bson.M{"$ne": true}},
				{"scope": bson.M{"$exists": false}, "isSystem": bson.M{"$ne": true}},
			},
		}
	}
	return bson.M{
		"projectId": bson.M{"$in": projectIDs},
		"$or": []bson.M{
			{"scope": "project"},
			{"scope": "", "isSystem": bson.M{"$ne": true}},
			{"scope": bson.M{"$exists": false}, "isSystem": bson.M{"$ne": true}},
		},
	}
}

func (d *DevopsCredential) Secret() (CredentialSecret, error) {
	if d == nil {
		return CredentialSecret{}, nil
	}
	secret := CredentialSecret{Username: d.Username}
	var err error
	if secret.Password, err = decryptSecret(d.Password); err != nil {
		return CredentialSecret{}, err
	}
	if secret.Token, err = decryptSecret(d.Token); err != nil {
		return CredentialSecret{}, err
	}
	if secret.PrivateKey, err = decryptSecret(d.PrivateKey); err != nil {
		return CredentialSecret{}, err
	}
	if secret.Passphrase, err = decryptSecret(d.Passphrase); err != nil {
		return CredentialSecret{}, err
	}
	if secret.Kubeconfig, err = decryptSecret(d.Kubeconfig); err != nil {
		return CredentialSecret{}, err
	}
	if secret.SecretText, err = decryptSecret(d.SecretText); err != nil {
		return CredentialSecret{}, err
	}
	if secret.Certificate, err = decryptSecret(d.Certificate); err != nil {
		return CredentialSecret{}, err
	}
	if secret.JsonData, err = decryptSecret(d.JsonData); err != nil {
		return CredentialSecret{}, err
	}
	return secret, nil
}

func encryptCredential(data *DevopsCredential) error {
	var err error
	if data.Password, err = encryptSecret(data.Password); err != nil {
		return err
	}
	if data.Token, err = encryptSecret(data.Token); err != nil {
		return err
	}
	if data.PrivateKey, err = encryptSecret(data.PrivateKey); err != nil {
		return err
	}
	if data.Passphrase, err = encryptSecret(data.Passphrase); err != nil {
		return err
	}
	if data.Kubeconfig, err = encryptSecret(data.Kubeconfig); err != nil {
		return err
	}
	if data.SecretText, err = encryptSecret(data.SecretText); err != nil {
		return err
	}
	if data.Certificate, err = encryptSecret(data.Certificate); err != nil {
		return err
	}
	if data.JsonData, err = encryptSecret(data.JsonData); err != nil {
		return err
	}
	return nil
}
