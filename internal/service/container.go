package service

// Container holds all service dependencies.
type Container struct {
	Config ConfigService
	PG     PGService
}

// NewContainer creates a service container.
func NewContainer(config ConfigService, pg PGService) *Container {
	return &Container{Config: config, PG: pg}
}

// Close closes all services.
func (c *Container) Close() error {
	var lastErr error
	if c.Config != nil {
		if err := c.Config.Close(); err != nil {
			lastErr = err
		}
	}
	if c.PG != nil {
		if err := c.PG.Disconnect(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
