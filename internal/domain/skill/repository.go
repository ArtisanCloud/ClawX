package skill

import "context"

type RegistryRepository interface {
	Upsert(ctx context.Context, metadata SkillMetadata) error
	GetByID(ctx context.Context, skillID string) (SkillMetadata, error)
	List(ctx context.Context) ([]SkillMetadata, error)
	Delete(ctx context.Context, skillID string) error
}

type PolicyRepository interface {
	Load(ctx context.Context) (SkillPolicy, error)
	Save(ctx context.Context, policy SkillPolicy) error
}

type BindingRepository interface {
	Upsert(ctx context.Context, binding SkillBinding) error
	Delete(ctx context.Context, binding SkillBinding) error
	List(ctx context.Context) ([]SkillBinding, error)
}
