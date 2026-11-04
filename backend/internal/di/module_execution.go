package di

import (
	"blotato-comment-automation/internal/feature/delivery"
	"blotato-comment-automation/internal/feature/execution"
	"blotato-comment-automation/internal/feature/ingress"
)

type ExecutionModule struct {
	*Services
	store   *execution.Store
	engine  *execution.Engine
	worker  *execution.Worker
	factory *ingress.Factory
}

func (s *Services) Execution() *ExecutionModule {
	if s.execution == nil {
		s.execution = &ExecutionModule{Services: s}
	}
	return s.execution
}
func (m *ExecutionModule) Store() (*execution.Store, error) {
	if m.store != nil {
		return m.store, nil
	}
	database, err := m.DB()
	if err != nil {
		return nil, err
	}
	m.store = execution.NewStore(database.GetPool())
	return m.store, nil
}
func (m *ExecutionModule) Engine() (*execution.Engine, error) {
	if m.engine != nil {
		return m.engine, nil
	}
	store, err := m.Store()
	if err != nil {
		return nil, err
	}
	m.engine = execution.NewEngine(store, m.Config().Execution.MaxAttempts)
	return m.engine, nil
}
func (m *ExecutionModule) Worker() (*execution.Worker, error) {
	if m.worker != nil {
		return m.worker, nil
	}
	store, err := m.Store()
	if err != nil {
		return nil, err
	}
	engine, err := m.Engine()
	if err != nil {
		return nil, err
	}
	transport, err := m.Kafka().Publisher()
	if err != nil {
		return nil, err
	}
	var sender delivery.Sender
	cfg := m.Config().Delivery
	if cfg.Mode == "recording" {
		database, err := m.DB()
		if err != nil {
			return nil, err
		}
		sender = delivery.NewRecorder(database.GetPool())
	} else {
		sender = delivery.NewHTTP(cfg)
	}
	m.worker = execution.NewWorker(store, engine, sender, execution.NewPublisher(transport), m.Config().Execution)
	return m.worker, nil
}
func (m *ExecutionModule) Factory() (*ingress.Factory, error) {
	if m.factory != nil {
		return m.factory, nil
	}
	comments, err := m.Inbound().Handler()
	if err != nil {
		return nil, err
	}
	store, err := m.Store()
	if err != nil {
		return nil, err
	}
	m.factory = ingress.NewFactory(comments, store, m.Config().Conversations)
	return m.factory, nil
}
