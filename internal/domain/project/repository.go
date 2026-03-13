package project

import "context"

type RegistryRepository interface {
	Get(ctx context.Context) (Registry, error)
	Save(ctx context.Context, registry Registry) error
}

type BindingRepository interface {
	GetByRouteKey(ctx context.Context, routeKey string) (RouteBinding, error)
	Upsert(ctx context.Context, binding RouteBinding) error
	DeleteByRouteKey(ctx context.Context, routeKey string) error
	List(ctx context.Context) ([]RouteBinding, error)
}

type ProposalRepository interface {
	GetByID(ctx context.Context, proposalID string) (Proposal, error)
	Upsert(ctx context.Context, proposal Proposal) error
	List(ctx context.Context) ([]Proposal, error)
}
