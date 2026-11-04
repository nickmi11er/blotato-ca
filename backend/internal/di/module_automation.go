package di

import (
	"context"
	"net/http"
	"time"

	"blotato-comment-automation/internal/feature/automation"
	"blotato-comment-automation/internal/feature/matching"
)

type AutomationModule struct {
	*Services
	service *automation.Service
	cache   *matching.Cache
	changes *automation.Changes
	server  *http.Server
}

func (s *Services) Automation() *AutomationModule {
	if s.automation == nil {
		s.automation = &AutomationModule{Services: s}
	}
	return s.automation
}
func (m *AutomationModule) Service() (*automation.Service, error) {
	if m.service != nil {
		return m.service, nil
	}
	database, err := m.DB()
	if err != nil {
		return nil, err
	}
	m.service = automation.NewService(database.GetPool())
	return m.service, nil
}
func (m *AutomationModule) Cache() (*matching.Cache, error) {
	if m.cache != nil {
		return m.cache, nil
	}
	database, err := m.DB()
	if err != nil {
		return nil, err
	}
	cache, err := matching.NewCache(m.Ctx(), matching.NewRepository(database.GetPool()), m.Config().Matching)
	if err != nil {
		return nil, err
	}
	m.Gc().Add(func(context.Context) error { cache.Close(); return nil })
	m.cache = cache
	return cache, nil
}
func (m *AutomationModule) Changes() (*automation.Changes, error) {
	if m.changes != nil {
		return m.changes, nil
	}
	database, err := m.DB()
	if err != nil {
		return nil, err
	}
	cache, err := m.Cache()
	if err != nil {
		return nil, err
	}
	bus, err := m.Kafka().Rules()
	if err != nil {
		return nil, err
	}
	m.changes = automation.NewChanges(database.GetPool(), bus, cache)
	return m.changes, nil
}
func (m *AutomationModule) HTTP() (*http.Server, error) {
	if m.server != nil {
		return m.server, nil
	}
	service, err := m.Service()
	if err != nil {
		return nil, err
	}
	cfg := m.Config().HTTP
	server := &http.Server{Addr: cfg.Address, Handler: automation.NewHTTPHandler(service, cfg.Token), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	m.Gc().Add(server.Shutdown)
	m.server = server
	return server, nil
}
