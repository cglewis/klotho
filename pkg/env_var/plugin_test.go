package envvar

import (
	"strings"
	"testing"

	"github.com/klothoplatform/klotho/pkg/annotation"
	"github.com/klothoplatform/klotho/pkg/compiler/types"
	"github.com/klothoplatform/klotho/pkg/construct"
	"github.com/klothoplatform/klotho/pkg/lang/javascript"
	"github.com/stretchr/testify/assert"
)

func Test_envVarPlugin(t *testing.T) {
	type testResult struct {
		resource construct.Construct
		envVars  types.EnvironmentVariables
	}
	tests := []struct {
		name    string
		source  string
		want    testResult
		wantErr bool
	}{
		{
			name: "simple redis node",
			source: `
/*
* @klotho::persist {
*   id = "myRedisNode"
*   [environment_variables]
*     REDIS_NODE_HOST = "redis_node.host"
*     REDIS_NODE_PORT = "redis_node.port"
* }
*/
const a = 1`,
			want: testResult{
				resource: &types.RedisNode{Name: "myRedisNode"},
				envVars: types.EnvironmentVariables{
					{
						Name:      "REDIS_NODE_HOST",
						Construct: &types.RedisNode{Name: "myRedisNode"},
						Value:     "host",
					},
					{
						Name:      "REDIS_NODE_PORT",
						Construct: &types.RedisNode{Name: "myRedisNode"},
						Value:     "port",
					},
				},
			},
		},
		{
			name: "simple redis cluster",
			source: `
/*
* @klotho::persist {
*   id = "myRedisCluster"
*   [environment_variables]
*     REDIS_HOST = "redis_cluster.host"
*     REDIS_PORT = "redis_cluster.port"
* }
*/
const a = 1`,
			want: testResult{
				resource: &types.RedisCluster{Name: "myRedisCluster"},
				envVars: types.EnvironmentVariables{
					{
						Name:      "REDIS_HOST",
						Construct: &types.RedisCluster{Name: "myRedisCluster"},
						Value:     "host",
					},
					{
						Name:      "REDIS_PORT",
						Construct: &types.RedisCluster{Name: "myRedisCluster"},
						Value:     "port",
					},
				},
			},
		},
		{
			name: "simple orm",
			source: `
/*
* @klotho::persist {
*   id = "myOrm"
*   [environment_variables]
*     ORM_CONNECTION_STRING = "orm.connection_string"
* }
*/
const a = 1`,
			want: testResult{
				resource: &types.Orm{Name: "myOrm"},
				envVars: types.EnvironmentVariables{
					{
						Name:      "ORM_CONNECTION_STRING",
						Construct: &types.Orm{Name: "myOrm"},
						Value:     "connection_string",
					},
				},
			},
		},
		{
			name: "error no id",
			source: `
/*
* @klotho::persist {
*   [environment_variables]
*     REDIS_HOST = "redis_cluster.host"
*     REDIS_PORT = "redis_cluster.port"
* }
*/
const a = 1`,
			wantErr: true,
		},
		{
			name: "error no environment variables",
			source: `
/*
* @klotho::persist {
*   [environment_variables]
* }
*/
const a = 1`,
			wantErr: true,
		},
		{
			name: "error invalid kind",
			source: `
/*
* @klotho::persist {
*   [environment_variables]
*     REDIS_HOST = "invalid.host"
* }
*/
const a = 1`,
			wantErr: true,
		},
		{
			name: "error invalid value",
			source: `
/*
* @klotho::persist {
*   [environment_variables]
*     REDIS_HOST = "redis_node.invalid"
* }
*/
const a = 1`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			f, err := javascript.NewFile("", strings.NewReader(tt.source))
			if !assert.NoError(err) {
				return
			}
			p := EnvVarInjection{}

			unit := &types.ExecutionUnit{Name: "unit"}
			unit.Add(f)
			result := construct.NewConstructGraph()
			result.AddConstruct(unit)
			err = p.Transform(&types.InputFiles{}, &types.FileDependencies{}, result)
			if tt.wantErr {
				assert.Error(err)
				return
			}
			if !assert.NoError(err) {
				return
			}

			resources := result.GetResourcesOfCapability(annotation.PersistCapability)
			assert.Len(resources, 1)
			assert.Equal(tt.want.resource, resources[0])

			downstreamDeps := result.GetDownstreamDependencies(unit)
			assert.Len(downstreamDeps, 1)
			assert.Equal(tt.want.resource.Id(), downstreamDeps[0].Destination.Id())

			assert.Len(unit.EnvironmentVariables, len(tt.want.envVars))
			for _, envVar := range tt.want.envVars {
				for _, unitVar := range unit.EnvironmentVariables {
					if envVar.Name == unitVar.Name {
						assert.Equal(envVar.Name, unitVar.Name)
						assert.Equal(envVar.Value, unitVar.Value)
						assert.Equal(envVar.Construct.Id(), unitVar.Construct.Id())

					}
				}
			}

		})
	}
}

func Test_parseDirectiveToEnvVars(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		want    EnvironmentVariableDirectiveResult
		wantErr bool
	}{
		{
			name: "simple happy path",
			source: `
/*
* @klotho::persist {
*   id = "myRedisNode"
*   [environment_variables]
*     REDIS_NODE_HOST = "redis_node.host"
*     REDIS_NODE_PORT = "redis_node.port"
* }
*/
const a = 1`,
			want: EnvironmentVariableDirectiveResult{
				kind: "redis_node",
				variables: types.EnvironmentVariables{
					{
						Name:  "REDIS_NODE_HOST",
						Value: "host",
					},
					{
						Name:  "REDIS_NODE_PORT",
						Value: "port",
					},
				},
			},
		},
		{
			name: "kind mistmatch",
			source: `
/*
* @klotho::persist {
*   id = "myRedisCluster"
*   [environment_variables]
*     REDIS_HOST = "redis_cluster.host"
*     REDIS_PORT = "redis_node.port"
* }
*/
const a = 1`,
			wantErr: true,
		},
		{
			name: "invalid env value",
			source: `
/*
* @klotho::persist {
*   id = "myRedisCluster"
*   [environment_variables]
*     REDIS_HOST = "redis_cluster.host.thisisnotallowed"
* }
*/
const a = 1`,
			wantErr: true,
		},
		{
			name: "error invalid kind",
			source: `
/*
* @klotho::persist {
*   [environment_variables]
*     REDIS_HOST = "invalid.host"
* }
*/
const a = 1`,
			wantErr: true,
		},
		{
			name: "error invalid value",
			source: `
/*
* @klotho::persist {
*   [environment_variables]
*     REDIS_HOST = "redis_node.invalid"
* }
*/
const a = 1`,
			wantErr: true,
		},
		{
			name: "error invalid value and kind for one env var",
			source: `
/*
* @klotho::persist {
*   [environment_variables]
*     REDIS_HOST = "redis_node.host"
*     INVALID = "invalid.invalid"
* }
*/
const a = 1`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			f, err := javascript.NewFile("", strings.NewReader(tt.source))
			if !assert.NoError(err) {
				return
			}
			var annot *types.Annotation
			for _, v := range f.Annotations() {
				annot = v
				break
			}
			cap := annot.Capability
			result, err := ParseDirectiveToEnvVars(cap)
			if tt.wantErr {
				assert.Error(err)
				return
			}
			if !assert.NoError(err) {
				return
			}

			assert.Equal(tt.want.kind, result.kind)

			assert.Len(result.variables, len(tt.want.variables))
			for _, envVar := range tt.want.variables {
				for _, unitVar := range result.variables {
					if envVar.Name == unitVar.Name {
						assert.Equal(envVar, unitVar)
					}
				}
			}

		})
	}
}
