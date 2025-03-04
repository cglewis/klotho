package imports

import (
	"github.com/klothoplatform/klotho/pkg/config"
	"github.com/klothoplatform/klotho/pkg/construct"
	"go.uber.org/zap"
)

// Plugin is responsible for adding the `Imported` resource to the graph, and adding a dependency from the resource
// which is to be replaced with the import. This mechanism for importing will work for all providers, so it is accomplished
// here in a generic plugin to prevent unnecessary reimplementation.
// If any special coordinating logic is required for a specific provider or resource, that should be implemented in the provider plugin.
type Plugin struct {
	Config *config.Application
}

func (p Plugin) Name() string {
	return "imports"
}

func (p Plugin) Translate(result *construct.ConstructGraph, dag *construct.ResourceGraph) error {
	log := zap.S()
	for resId, importId := range p.Config.Imports {
		res := dag.GetResource(resId)
		if res == nil {
			log.Warnf("No resource found for import '%s'", resId)
			continue
		}
		dag.AddDependency(res, &Imported{ID: importId})
	}
	return nil
}

func (p Plugin) LoadGraph(graph construct.OutputGraph, dag *construct.ConstructGraph) error {
	return nil
}
