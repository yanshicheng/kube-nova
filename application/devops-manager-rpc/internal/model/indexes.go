package model

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (m *DevopsStepTemplateModel) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "engineType", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_step_template_engine_created"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "engineType", Value: 1}, {Key: "categoryId", Value: 1}, {Key: "type", Value: 1}, {Key: "status", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_step_template_list"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "engineType", Value: 1}, {Key: "code", Value: 1}},
			Options: options.Index().SetName("idx_step_template_engine_code"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "categoryId", Value: 1}},
			Options: options.Index().SetName("idx_step_template_category"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "engineType", Value: 1}, {Key: "params.code", Value: 1}},
			Options: options.Index().SetName("idx_step_template_param_code"),
		},
	})
	return err
}

func (m *DevopsStepCategoryModel) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "sortOrder", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_step_category_sort"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "status", Value: 1}, {Key: "sortOrder", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_step_category_status_sort"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "code", Value: 1}},
			Options: options.Index().SetName("idx_step_category_code"),
		},
	})
	return err
}

func (m *DevopsChannelGroupModel) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "code", Value: 1}},
			Options: options.Index().SetName("idx_channel_group_code"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "status", Value: 1}, {Key: "sortOrder", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_channel_group_list"),
		},
	})
	return err
}

func (m *DevopsChannelTypeModel) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "code", Value: 1}},
			Options: options.Index().SetName("idx_channel_type_code"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "groupCode", Value: 1}, {Key: "status", Value: 1}, {Key: "code", Value: 1}},
			Options: options.Index().SetName("idx_channel_type_group_list"),
		},
	})
	return err
}

func (m *DevopsPipelineTemplateModel) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "engineType", Value: 1}, {Key: "status", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_pipeline_template_engine_created"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "scope", Value: 1}, {Key: "projectId", Value: 1}, {Key: "engineType", Value: 1}, {Key: "status", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_pipeline_template_scope_project"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "projectId", Value: 1}, {Key: "engineType", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_pipeline_template_project_created"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "code", Value: 1}},
			Options: options.Index().SetName("idx_pipeline_template_code"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "steps.stepId", Value: 1}},
			Options: options.Index().SetName("idx_pipeline_template_step"),
		},
	})
	return err
}

func (m *DevopsProjectMemberModel) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "userId", Value: 1}, {Key: "status", Value: 1}, {Key: "projectId", Value: 1}},
			Options: options.Index().SetName("idx_project_member_user_project"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "projectId", Value: 1}, {Key: "role", Value: 1}, {Key: "status", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_project_member_list"),
		},
	})
	return err
}

func (m *DevopsProjectChannelBindingModel) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "projectId", Value: 1}, {Key: "channelGroupCode", Value: 1}, {Key: "channelType", Value: 1}, {Key: "status", Value: 1}, {Key: "isDefault", Value: -1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_project_channel_group_list"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "projectId", Value: 1}, {Key: "usageScope", Value: 1}, {Key: "channelType", Value: 1}, {Key: "status", Value: 1}, {Key: "isDefault", Value: -1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_project_channel_scope_list"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "projectId", Value: 1}, {Key: "channelId", Value: 1}, {Key: "status", Value: 1}},
			Options: options.Index().SetName("idx_project_channel_active"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "projectCredentialId", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_project_channel_credential"),
		},
	})
	return err
}

func (m *DevopsProjectConfigModel) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "projectId", Value: 1}, {Key: "typeCode", Value: 1}, {Key: "status", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_project_config_list"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "projectId", Value: 1}, {Key: "typeCode", Value: 1}, {Key: "code", Value: 1}},
			Options: options.Index().SetName("idx_project_config_type_code"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "projectId", Value: 1}, {Key: "typeCode", Value: 1}, {Key: "name", Value: 1}},
			Options: options.Index().SetName("idx_project_config_type_name"),
		},
	})
	return err
}

func (m *DevopsChannelModel) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "isSystem", Value: 1}, {Key: "groupId", Value: 1}, {Key: "channelType", Value: 1}, {Key: "status", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_channel_list"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "code", Value: 1}},
			Options: options.Index().SetName("idx_channel_code"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "credentialId", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_channel_credential"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "globalCredentialId", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_channel_global_credential"),
		},
	})
	return err
}

func (m *DevopsCredentialModel) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "credentialType", Value: 1}, {Key: "status", Value: 1}, {Key: "projectId", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_credential_type_project"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "scope", Value: 1}, {Key: "isSystem", Value: 1}, {Key: "projectId", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_credential_scope_project"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "code", Value: 1}},
			Options: options.Index().SetName("idx_credential_code"),
		},
	})
	return err
}

func (m *DevopsStepChannelParamModel) EnsureIndexes(ctx context.Context) error {
	_, err := m.conn.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "status", Value: 1}, {Key: "sortOrder", Value: 1}, {Key: "createAt", Value: -1}},
			Options: options.Index().SetName("idx_step_channel_param_active"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "paramType", Value: 1}},
			Options: options.Index().SetName("idx_step_channel_param_type"),
		},
		{
			Keys:    bson.D{{Key: "isDeleted", Value: 1}, {Key: "paramType", Value: 1}, {Key: "groupCode", Value: 1}, {Key: "channelTypeFilter", Value: 1}},
			Options: options.Index().SetName("idx_step_channel_param_target"),
		},
	})
	return err
}
