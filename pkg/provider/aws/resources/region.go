package resources

import (
	"fmt"

	"github.com/klothoplatform/klotho/pkg/core"
)

const REGION_TYPE = "region"
const AVAILABILITY_ZONES_TYPE = "availability_zones"
const ACCOUNT_ID_TYPE = "account_id"
const ARN_IAC_VALUE = "arn"

type (
	Region struct {
		Name          string
		ConstructsRef []core.AnnotationKey
	}

	AvailabilityZones struct {
		Name          string
		ConstructsRef []core.AnnotationKey
	}

	AccountId struct {
		Name          string
		ConstructsRef []core.AnnotationKey
	}
)

func NewRegion() *Region {
	return &Region{
		Name:          "region",
		ConstructsRef: []core.AnnotationKey{},
	}
}

// Provider returns name of the provider the resource is correlated to
func (region *Region) Provider() string {
	return AWS_PROVIDER
}

// KlothoConstructRef returns AnnotationKey of the klotho resource the cloud resource is correlated to
func (region *Region) KlothoConstructRef() []core.AnnotationKey {
	return region.ConstructsRef
}

// Id returns the id of the cloud resource
func (region *Region) Id() string {
	return fmt.Sprintf("%s:%s:%s", region.Provider(), REGION_TYPE, region.Name)
}

func NewAvailabilityZones() *AvailabilityZones {
	return &AvailabilityZones{
		Name:          "AvailabilityZones",
		ConstructsRef: []core.AnnotationKey{},
	}
}

// Provider returns name of the provider the resource is correlated to
func (azs *AvailabilityZones) Provider() string {
	return AWS_PROVIDER
}

// KlothoConstructRef returns AnnotationKey of the klotho resource the cloud resource is correlated to
func (azs *AvailabilityZones) KlothoConstructRef() []core.AnnotationKey {
	return azs.ConstructsRef
}

// Id returns the id of the cloud resource
func (azs *AvailabilityZones) Id() string {
	return fmt.Sprintf("%s:%s:%s", azs.Provider(), AVAILABILITY_ZONES_TYPE, azs.Name)
}

func NewAccountId() *AccountId {
	return &AccountId{
		Name:          "AccountId",
		ConstructsRef: []core.AnnotationKey{},
	}
}

// Provider returns name of the provider the resource is correlated to
func (id *AccountId) Provider() string {
	return AWS_PROVIDER
}

// KlothoConstructRef returns AnnotationKey of the klotho resource the cloud resource is correlated to
func (id *AccountId) KlothoConstructRef() []core.AnnotationKey {
	return id.ConstructsRef
}

// Id returns the id of the cloud resource
func (id *AccountId) Id() string {
	return fmt.Sprintf("%s:%s:%s", id.Provider(), ACCOUNT_ID_TYPE, id.Name)
}
