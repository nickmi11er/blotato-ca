package di

import (
	"blotato-comment-automation/internal/feature/inbound"
)

type InboundModule struct {
	*Services

	repository inbound.Repository
	handler    *inbound.Handler
}

func (s *Services) Inbound() *InboundModule {
	if s.inbound == nil {
		s.inbound = &InboundModule{Services: s}
	}
	return s.inbound
}

func (m *InboundModule) Handler() (*inbound.Handler, error) {
	if m.handler != nil {
		return m.handler, nil
	}
	repository, err := m.Repository()
	if err != nil {
		return nil, err
	}
	m.handler = inbound.NewHandler(repository)
	return m.handler, nil
}

func (m *InboundModule) Repository() (inbound.Repository, error) {
	if m.repository != nil {
		return m.repository, nil
	}
	database, err := m.DB()
	if err != nil {
		return nil, err
	}
	m.repository = inbound.NewRepository(database)
	return m.repository, nil
}
