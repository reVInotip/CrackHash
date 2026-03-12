package configuration

import (
	"maps"
	"log"
	"runtime"
)

type Config struct {
	params map[string]Parameter
}

type ConfigSource struct {
	Name string
	CreateHandle func() *Config
	config *Config
}


var ConfigurationSources []ConfigSource

var GlobalConfig *Config = nil

func NewConfig() *Config {
	return &Config{
		params: make(map[string]Parameter),
	}
}

func InitGlobalConfig() {
	for i := range ConfigurationSources {
		log.Printf("Start parsing config data from source %s", ConfigurationSources[i].Name)
		if ConfigurationSources[i].config == nil {
			ConfigurationSources[i].config = ConfigurationSources[i].CreateHandle()
		}
	}
	GlobalConfig = MergeConfigsFromAnySources()
	UnlinkMemoryStructs()
}

func MergeConfigsFromAnySources() *Config {
	if len(ConfigurationSources) == 0 {
		return NewConfig()
	}
	// Start with first source
	merged := NewConfig()
	maps.Copy(merged.params, ConfigurationSources[0].config.params)
	// Overlay with subsequent sources
	for i := 1; i < len(ConfigurationSources); i++ {
		maps.Copy(merged.params, ConfigurationSources[i].config.params)
	}
	return merged
}

func UnlinkMemoryStructs() {
	for _, configSource := range ConfigurationSources {
		log.Printf("Unlink config from source %s", configSource.Name)
		configSource.config = nil
	}
	runtime.GC()
}

func AddConfParam[T any](c *Config, name string, defaultValue T) {
	if c.params == nil {
		c.params = make(map[string]Parameter)
	}
	c.params[name] = &ConfigParam[T]{
		name:  name,
		value: defaultValue,
	}
}

func GetConfParam[T any](c *Config, name string) (T, bool) {
	if p, ok := c.params[name]; ok {
		if typed, ok := p.(*ConfigParam[T]); ok {
			return typed.Get(), true
		}
	}
	var zero T
	return zero, false
}