// Package registry manages the lifecycle of all messaging channel plugins.
package registry

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/highclaw/highclaw/pkg/pluginsdk"
)

// Registry manages all registered channel plugins.
type Registry struct {
	logger   *slog.Logger
	mu       sync.RWMutex
	channels map[string]pluginsdk.Channel
	handler  pluginsdk.MessageHandler
}

// NewRegistry creates a new channel registry.
func NewRegistry(logger *slog.Logger, handler pluginsdk.MessageHandler) *Registry {
	return &Registry{
		logger:   logger.With("component", "channels"),
		channels: make(map[string]pluginsdk.Channel),
		handler:  handler,
	}
}

// Register adds a channel to the registry.
func (r *Registry) Register(ch pluginsdk.Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels[ch.Name()] = ch
	r.logger.Info("channel registered", "name", ch.Name())
}

// StartAll starts all registered channels.
func (r *Registry) StartAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, ch := range r.channels {
		r.logger.Info("starting channel", "name", name)
		if err := ch.Start(ctx); err != nil {
			r.logger.Error("channel start failed", "name", name, "error", err)
			// Don't fail the whole startup for one channel.
			continue
		}
		r.logger.Info("channel started", "name", name)
	}
	return nil
}

// StopAll stops all registered channels.
func (r *Registry) StopAll() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, ch := range r.channels {
		r.logger.Info("stopping channel", "name", name)
		if err := ch.Stop(); err != nil {
			r.logger.Error("channel stop error", "name", name, "error", err)
		}
	}
}

// Get returns a channel by name.
func (r *Registry) Get(name string) (pluginsdk.Channel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ch, ok := r.channels[name]
	if !ok {
		return nil, fmt.Errorf("channel not found: %s", name)
	}
	return ch, nil
}

// Status returns the connection status of all channels.
func (r *Registry) Status() map[string]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := make(map[string]bool, len(r.channels))
	for name, ch := range r.channels {
		status[name] = ch.IsConnected()
	}
	return status
}
