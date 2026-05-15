package configuration

import "sync"

type Parameter interface {
    Name() string
}

type ConfigParam[T any] struct {
	name string
	value T
	mu sync.RWMutex
}

func (p *ConfigParam[T]) Get() T {
	p.mu.RLock()
	defer p.mu.RUnlock()
    return p.value
}

func (p *ConfigParam[T]) Set(v T) {
	p.mu.Lock()
	defer p.mu.Unlock()
    p.value = v
}

func (p *ConfigParam[T]) Name() string {
    return p.name
}