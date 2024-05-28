package util

import "flag"

type EnvType int8

const (
	EnvTypeDev EnvType = iota
	EnvTypeTesting
	EnvTypeProd
)

var EnvTypeFromString = map[string]EnvType{
	"dev":  EnvTypeDev,
	"test": EnvTypeTesting,
	"prod": EnvTypeProd,
}

var RootEnvType string

func init() {
	flag.StringVar(&RootEnvType, "envtype", "dev", "dev|test|prod")
	flag.Parse()
}

func GetEnvType() EnvType {
	envType, _ := EnvTypeFromString[RootEnvType]
	return envType
}

func (e EnvType) String() string {
	for key, v := range EnvTypeFromString {
		if v == e {
			return key
		}
	}
	return "unknown"
}
