package model

import (
	"context"
	"errors"
)

func EnsureDefaultConfigCenterData(ctx context.Context, typeModel *DevopsConfigTypeModel, mavenModel *DevopsProjectMavenConfigModel, projectConfigModel *DevopsProjectConfigModel) error {
	if typeModel == nil || projectConfigModel == nil {
		return nil
	}
	configType, err := typeModel.FindOneByCode(ctx, DefaultMavenSettingsTypeCode)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		configType = &DevopsConfigType{
			Name:        "maven配置文件",
			Code:        DefaultMavenSettingsTypeCode,
			StorageType: "xml",
			Description: "Maven settings.xml 配置文件",
			SortOrder:   10,
			Status:      1,
			CreatedBy:   "system",
			UpdatedBy:   "system",
		}
		if err := typeModel.Insert(ctx, configType); err != nil && !isDuplicateKey(err) {
			return err
		}
	}
	if mavenModel == nil {
		return nil
	}
	return migrateMavenConfigsToConfigCenter(ctx, mavenModel, projectConfigModel, configType)
}

func migrateMavenConfigsToConfigCenter(ctx context.Context, mavenModel *DevopsProjectMavenConfigModel, projectConfigModel *DevopsProjectConfigModel, configType *DevopsConfigType) error {
	if configType == nil {
		return nil
	}
	page := uint64(1)
	for {
		items, _, err := mavenModel.List(ctx, DevopsProjectMavenConfigListFilter{
			Status:   -1,
			Page:     page,
			PageSize: 200,
		})
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		for _, item := range items {
			if item == nil || item.ProjectID == "" || item.Code == "" {
				continue
			}
			exist, err := projectConfigModel.FindOneByProjectTypeCode(ctx, item.ProjectID, DefaultMavenSettingsTypeCode, item.Code)
			if err != nil && !errors.Is(err, ErrNotFound) {
				return err
			}
			if exist != nil {
				continue
			}
			data := &DevopsProjectConfig{
				ProjectID:   item.ProjectID,
				TypeID:      configType.ID.Hex(),
				TypeCode:    DefaultMavenSettingsTypeCode,
				TypeName:    configType.Name,
				Name:        item.Name,
				Code:        item.Code,
				Content:     item.Content,
				Description: item.Description,
				Status:      item.Status,
				CreatedBy:   item.CreatedBy,
				UpdatedBy:   item.UpdatedBy,
			}
			if err := projectConfigModel.Insert(ctx, data); err != nil && !isDuplicateKey(err) {
				return err
			}
		}
		if len(items) < 200 {
			return nil
		}
		page++
	}
}
