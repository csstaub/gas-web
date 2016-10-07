package main

import (
	gas "github.com/HewlettPackard/gas/core"
)

func buildConfig() map[string]interface{} {
	config := map[string]interface{}{}
	config["ignoreNosec"] = false
	return config
}

func buildAnalyzer() *gas.Analyzer {
	config := buildConfig()
	analyzer := gas.NewAnalyzer(config, logger)
	addRules(&analyzer, config)
	return &analyzer
}
