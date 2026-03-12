package default_configs

import (
	"TaskOneUtils/configuration"
	"os"
	"strconv"
	"strings"
)

func NewEnvConfig() *configuration.Config {
	var conf configuration.Config
	
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		name := parts[0]
		value := parts[1]

		if decimal, err := strconv.Atoi(value); err == nil {
			configuration.AddConfParam(&conf, name, decimal)
		} else if number, err := strconv.ParseFloat(value, 64); err == nil {
			configuration.AddConfParam(&conf, name, number)
		} else {
			configuration.AddConfParam(&conf, name, value)
		}
	}

	return &conf
}