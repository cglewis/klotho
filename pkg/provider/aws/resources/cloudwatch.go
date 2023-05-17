package resources

import (
	"fmt"

	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/sanitization/aws"
)

const LOG_GROUP_TYPE = "log_group"

var logGroupSanitizer = aws.CloudwatchLogGroupSanitizer

type (
	LogGroup struct {
		Name            string
		ConstructsRef   []core.AnnotationKey
		LogGroupName    string
		RetentionInDays int
	}
)

type CloudwatchLogGroupCreateParams struct {
	AppName string
	Refs    []core.AnnotationKey
	Name    string
}

// Create takes in an all necessary parameters to generate the LogGroup name and ensure that the LogGroup is correlated to the constructs which required its creation.
func (logGroup *LogGroup) Create(dag *core.ResourceGraph, params CloudwatchLogGroupCreateParams) error {
	logGroup.Name = logGroupSanitizer.Apply(fmt.Sprintf("%s-%s", params.AppName, params.Name))
	logGroup.ConstructsRef = params.Refs

	existingLogGroup := dag.GetResource(logGroup.Id())
	if existingLogGroup != nil {
		graphLogGroup := existingLogGroup.(*LogGroup)
		graphLogGroup.ConstructsRef = core.DedupeAnnotationKeys(append(graphLogGroup.KlothoConstructRef(), params.Refs...))
	} else {
		dag.AddResource(logGroup)
	}
	return nil
}

func NewLogGroup(appName string, logGroupName string, ref core.AnnotationKey, retention int) *LogGroup {
	return &LogGroup{
		Name:            logGroupSanitizer.Apply(fmt.Sprintf("%s-%s", appName, logGroupName)),
		ConstructsRef:   []core.AnnotationKey{ref},
		LogGroupName:    logGroupSanitizer.Apply(logGroupName),
		RetentionInDays: retention,
	}
}

// KlothoConstructRef returns AnnotationKey of the klotho resource the cloud resource is correlated to
func (lg *LogGroup) KlothoConstructRef() []core.AnnotationKey {
	return lg.ConstructsRef
}

// Id returns the id of the cloud resource
func (lg *LogGroup) Id() core.ResourceId {
	return core.ResourceId{
		Provider: AWS_PROVIDER,
		Type:     LOG_GROUP_TYPE,
		Name:     lg.Name,
	}
}
