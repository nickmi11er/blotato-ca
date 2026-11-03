package di

import "blotato-comment-automation/internal/config"

func (s *Services) Config() config.Config { return s.cfg }
