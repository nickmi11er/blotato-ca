package di

import (
	"blotato-comment-automation/internal/feature/admission"
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
	cache, err := m.Automation().Cache()
	if err != nil {
		return nil, err
	}
	m.handler = inbound.NewHandler(repository, cache)
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
	m.repository = admission.NewRepository(database.GetPool())
	return m.repository, nil
}
