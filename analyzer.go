package main

import (
	gas "github.com/HewlettPackard/gas/core"
)

func buildConfig() map[string]interface{} {
	config := map[string]interface{}{}
	config["include"] = []string{}
	config["exclude"] = []string{}
	config["ignoreNosec"] = false
	return config
}

func buildAnalyzer(config map[string]interface{}) *gas.Analyzer {
	analyzer := gas.NewAnalyzer(config, logger)
	AddRules(&analyzer, config)
	return &analyzer
}
