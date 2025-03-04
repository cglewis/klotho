package resources

import (
	"testing"

	"github.com/klothoplatform/klotho/pkg/compiler/types"
	"github.com/klothoplatform/klotho/pkg/construct"
	"github.com/klothoplatform/klotho/pkg/construct/coretesting"
	"github.com/stretchr/testify/assert"
)

func Test_SecretCreate(t *testing.T) {
	eu := &types.ExecutionUnit{Name: "test"}
	cases := []coretesting.CreateCase[SecretCreateParams, *Secret]{
		{
			Name: "nil igw",
			Want: coretesting.ResourcesExpectation{
				Nodes: []string{
					"aws:secret:my-app-secret",
				},
				Deps: []coretesting.StringDep{},
			},
			Check: func(assert *assert.Assertions, s *Secret) {
				assert.Equal(s.Name, "my-app-secret")
				assert.Equal(s.ConstructRefs, construct.BaseConstructSetOf(eu))
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.Name, func(t *testing.T) {
			tt.Params = SecretCreateParams{
				AppName: "my-app",
				Refs:    construct.BaseConstructSetOf(eu),
				Name:    "secret",
			}
			tt.Run(t)
		})
	}
}

func Test_SecretVersionCreate(t *testing.T) {
	eu := &types.ExecutionUnit{Name: "test"}
	cases := []coretesting.CreateCase[SecretVersionCreateParams, *SecretVersion]{
		{
			Name: "nil igw",
			Want: coretesting.ResourcesExpectation{
				Nodes: []string{
					"aws:secret_version:my-app-secret",
				},
				Deps: []coretesting.StringDep{},
			},
			Check: func(assert *assert.Assertions, sv *SecretVersion) {
				assert.Equal(sv.Name, "my-app-secret")
				assert.Equal(sv.ConstructRefs, construct.BaseConstructSetOf(eu))
				assert.Equal(sv.DetectedPath, "path")
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.Name, func(t *testing.T) {
			tt.Params = SecretVersionCreateParams{
				AppName:      "my-app",
				Refs:         construct.BaseConstructSetOf(eu),
				Name:         "secret",
				DetectedPath: "path",
			}
			tt.Run(t)
		})
	}
}
