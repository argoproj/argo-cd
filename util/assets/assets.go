package assets

import (
	"github.com/argoproj/argo-cd/v2/assets"
)

var (
	BuiltinPolicyCSV string
	ModelConf        string
	SwaggerJSON      string
	BadgeSVG         string
)

func init() {
	data, err := assets.Embedded.ReadFile("builtin-policy.csv")
	if err != nil {
		panic(err)
	}
	BuiltinPolicyCSV = string(data)
	data, err = assets.Embedded.ReadFile("model.conf")
	if err != nil {
		panic(err)
	}
	ModelConf = string(data)
	data, err = assets.Embedded.ReadFile("swagger.json")
	if err != nil {
		panic(err)
	}
	SwaggerJSON = string(data)
	data, err = assets.Embedded.ReadFile("badge.svg")
	if err != nil {
		panic(err)
	}
	BadgeSVG = string(data)
}
