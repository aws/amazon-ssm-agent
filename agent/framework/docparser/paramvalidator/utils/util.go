package utils

import (
	"encoding/json"
	"math"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

// DocumentParamType represents various document parameter types
type DocumentParamType string

const (
	// ParamTypeString represents the Param Type String
	ParamTypeString DocumentParamType = "String"
	// ParamTypeStringList represents the Param Type StringList
	ParamTypeStringList DocumentParamType = "StringList"
	// ParamTypeStringMap represents the param type StringMap
	ParamTypeStringMap DocumentParamType = "StringMap"
	// ParamTypeInteger represents the param type StringMap
	ParamTypeInteger DocumentParamType = "Integer"
	// ParamTypeBoolean represents the param type Boolean
	ParamTypeBoolean DocumentParamType = "Boolean"
	// ParamTypeMapList represents the param type MapList
	ParamTypeMapList DocumentParamType = "MapList"
)

// GetCleanedUpVal verifies the parameter value is within the limits and
// returns defaultVal if invalid parameter value passed to this function
func GetCleanedUpVal(log log.T, parameter json.Number, defaultVal int) int {
	itemVal, parseErr := parameter.Int64()
	if parseErr != nil || itemVal > math.MaxInt32 || itemVal < 0 {
		log.Warnf("cleaning up parameter value /%v/ so returning default value /%v/", parameter, defaultVal)
		return defaultVal
	} else {
		return int(itemVal)
	}
}
