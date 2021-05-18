package assets

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/gobuffalo/packr"
)

var (
	BuiltinPolicyCSV string
	ModelConf        string
	SwaggerJSON      string
	BadgeSVG         string
)

func init() {
	var err error
	box := packr.NewBox("../../assets")
	BuiltinPolicyCSV, err = box.MustString("builtin-policy.csv")
	if err != nil {
		panic(err)
	}
	// Possible upgrade - use viper to specify file location rather than hardcoded here and helmchart.
	if _, err := os.Stat("/etc/assets/model.conf"); err == nil {
		model, err := ioutil.ReadFile("/etc/assets/model.conf")
		if err != nil {
			fmt.Println("File reading error", err)
			return
		}
		ModelConf = string(model)
	} else if os.IsNotExist(err) {
		ModelConf, err = box.MustString("model.conf")
		if err != nil {
			panic(err)
		}
	} else {
		panic(err)
	}
	SwaggerJSON, err = box.MustString("swagger.json")
	if err != nil {
		panic(err)
	}
	BadgeSVG, err = box.MustString("badge.svg")
	if err != nil {
		panic(err)
	}
}
