package python

import (
	"github.com/klothoplatform/klotho/pkg/config"
	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/multierr"
)

type (
	AddExecRuntimeFiles struct {
		runtime Runtime
		cfg     *config.Application
	}
)

func (p *AddExecRuntimeFiles) Name() string { return "AddExecRuntimeFiles:Python" }

func (p *AddExecRuntimeFiles) Transform(input *core.InputFiles, fileDeps *core.FileDependencies, constructGraph *core.ConstructGraph) error {
	var errs multierr.Error
	for _, unit := range core.GetConstructsOfType[*core.ExecutionUnit](constructGraph) {
		if !unit.HasSourceFilesFor(Language.ID) {
			continue
		}
		errs.Append(p.runtime.AddExecRuntimeFiles(unit, constructGraph))
	}

	return errs.ErrOrNil()
}
