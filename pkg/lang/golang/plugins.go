package golang

import (
	"github.com/klothoplatform/klotho/pkg/compiler"
	"github.com/klothoplatform/klotho/pkg/compiler/types"
	"github.com/klothoplatform/klotho/pkg/config"
	"github.com/klothoplatform/klotho/pkg/construct"
	"go.uber.org/zap"
)

type (
	GoPlugins struct {
		Plugins []compiler.AnalysisAndTransformationPlugin
	}
)

func NewGoPlugins(cfg *config.Application, runtime Runtime) *GoPlugins {
	return &GoPlugins{
		Plugins: []compiler.AnalysisAndTransformationPlugin{
			&Expose{Config: cfg, runtime: runtime},
			&AddExecRuntimeFiles{cfg: cfg, runtime: runtime},
			&PersistFsPlugin{runtime: runtime},
			&PersistSecretsPlugin{runtime: runtime, config: cfg},
		},
	}
}

func (c GoPlugins) Name() string { return "go" }

func (c GoPlugins) Transform(input *types.InputFiles, fileDeps *types.FileDependencies, constructGraph *construct.ConstructGraph) error {
	for _, p := range c.Plugins {
		log := zap.L().With(zap.String("plugin", p.Name()))
		log.Debug("starting")
		err := p.Transform(input, fileDeps, constructGraph)
		if err != nil {
			return types.NewPluginError(p.Name(), err)
		}
		log.Debug("completed")
	}

	return nil
}
