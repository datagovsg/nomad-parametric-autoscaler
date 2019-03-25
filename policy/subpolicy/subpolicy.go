package subpolicy

import (
	"fmt"

	"github.com/datagovsg/nomad-parametric-autoscaler/resources"
)

// ScalingMagnitude needs a way better name
type ScalingMagnitude struct {
	ChangeType  string  `json:"ChangeType"`
	ChangeValue float64 `json:"ChangeValue"`
}

type SubPolicy interface {
	RecommendCount() map[resources.Resource]int
	GetManagedResources() []resources.Resource
	DeriveGenericSubpolicy() GenericSubPolicy
}

// GenericSubPolicy is used for decoding of json
// according to name, create actual sp
type GenericSubPolicy struct {
	Name             string      `json:"Name"`
	ManagedResources []string    `json:"ManagedResources"`
	Metadata         interface{} `json:"Metadata"`
}

// CreateSpecificSubpolicy checks name of GSP and creates the actual policy
func CreateSpecificSubpolicy(gsp GenericSubPolicy, mr []resources.Resource) (SubPolicy, error) {
	switch gsp.Name {
	case "CoreRatio":
		sp, err := NewCoreRatioSubpolicy(gsp.Name, mr, gsp.Metadata)
		if err != nil {
			return nil, err
		}
		return sp, nil
	case "OfficeHour":
		sp, err := NewOfficeHourSubPolicy(gsp.Name, mr, gsp.Metadata)
		if err != nil {
			return nil, err
		}
		return sp, nil
	default:
		return nil, fmt.Errorf("%v is not a valid subpolicy", gsp.Name)
	}
}

// determineNewDesiredLevel is a utiliy function that resolves the various types
// of scaling methods
func determineNewDesiredLevel(cur int, sm ScalingMagnitude) int {
	switch sm.ChangeType {
	case "multiply":
		return int(float64(cur) * sm.ChangeValue)
	case "until":
		return int(sm.ChangeValue)
	default:
		return cur
	}
}
