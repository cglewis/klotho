package iac2

import (
	"fmt"
	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/infra/kubernetes"
	"github.com/klothoplatform/klotho/pkg/provider/aws/resources"
	"github.com/klothoplatform/klotho/pkg/provider/aws/resources/cloudwatch"
	"github.com/klothoplatform/klotho/pkg/provider/aws/resources/ecr"
	"github.com/klothoplatform/klotho/pkg/provider/aws/resources/iam"
	"github.com/klothoplatform/klotho/pkg/provider/aws/resources/lambda"
	"github.com/klothoplatform/klotho/pkg/provider/aws/resources/s3"
	"github.com/klothoplatform/klotho/pkg/provider/aws/resources/vpc"
	assert "github.com/stretchr/testify/assert"
	"go/types"
	"golang.org/x/tools/go/packages"
	"reflect"
	"strings"
	"testing"
)

type (
	Methods struct {
		// signatures is the set of all methods declared on a type. Each signature follows the general format:
		//
		//	<name> func(<args>) <return type>
		//
		// The args do not include the receiver type. For example:
		//
		//	KlothoConstructRef func() []github.com/klothoplatform/klotho/pkg/core.AnnotationKey
		signatures  map[string]struct{}
		isInterface bool
	}

	TypeRef struct {
		pkg  string
		name string
	}
)

// TestKnownTemplates performs several tests to make sure that our Go structs match up with the factory.ts templates.
//
// For each known type, it checks:
//   - That there's a template for that struct
//   - That the template has a valid "output" type
//   - That for each input defined in the template's Args: (a) there is a corresponding field in the Go struct, and (b)
//     that the Go struct's field type matches with the Arg field type.
//
// To do the field-matching, we look at the Go struct's field type, and compute from it the expected TypeScript type.
// For primitives (int, bool, string, etc) we just convert it to the corresponding TypeScript primitive. For structs,
// we look up that struct's template and expect whatever the output of that template is. See the [iac2] package docs
// for more ("Why a template for a template?").
//
// We don't check the template's return expression, because we assume that if it has a valid output type, it'll also
// have a valid return expression. (Otherwise, our separate tsc checks will fail.)
//
// With all that done, we also check that we've validated all the structs in pkg/provider/aws/. To do this,
// we use the reflective [packages.Load] to find all the types within that package, and then filter down to those types
// that conform to core.Resource. Then we simply check that each one of those is in the list of types we checked.
func TestKnownTemplates(t *testing.T) {
	var allResources = []core.Resource{
		&resources.Region{},
		&vpc.Vpc{},
		&vpc.VpcEndpoint{},
		&kubernetes.KlothoHelmChart{},
		&lambda.LambdaFunction{},
		&ecr.EcrImage{},
		&cloudwatch.LogGroup{},
		&vpc.ElasticIp{},
		&vpc.NatGateway{},
		&vpc.Subnet{},
		&vpc.InternetGateway{},
		&iam.IamRole{},
		&ecr.EcrRepository{},
		&s3.S3Bucket{},
		&resources.AccountId{},
	}

	tp := standardTemplatesProvider()
	testedTypes := make(map[TypeRef]struct{})
	for _, res := range allResources {
		resType := reflect.TypeOf(res)
		for resType.Kind() == reflect.Pointer {
			resType = resType.Elem()
		}
		testedTypes[TypeRef{pkg: resType.PkgPath(), name: resType.Name()}] = struct{}{}
		t.Run(resType.String(), func(t *testing.T) {
			var tmpl ResourceCreationTemplate

			tmplFound := t.Run("template exists", func(t *testing.T) {
				assert := assert.New(t)
				found, err := tp.getTemplate(res)
				if !assert.NoError(err) {
					return
				}
				tmpl = found
			})
			if !tmplFound {
				return
			}
			t.Run("output", func(t *testing.T) {
				assert := assert.New(t)
				assert.NotEmpty(tmpl.OutputType)
			})

			t.Run("inputs", func(t *testing.T) {
				for inputName, inputTsType := range tmpl.InputTypes {
					if inputName == "dependsOn" {
						continue
					}
					t.Run(inputName, func(t *testing.T) {
						assert := assert.New(t)

						field, fieldFound := resType.FieldByName(inputName)
						if !assert.Truef(fieldFound, `missing field`, field.Name) {
							return
						}
						assert.Truef(field.IsExported(), `field is not exported`, field.Name)

						expectedType := &strings.Builder{}
						if err := buildExpectedTsType(expectedType, tp, field.Type); !assert.NoError(err) {
							return
						}
						assert.NotEmpty(expectedType, `couldn't determine expected type'`)
						assert.Equal(expectedType.String(), inputTsType, `field type`)
					})
				}
			})
		})
	}
	t.Run("all types tested", func(t *testing.T) {
		assert := assert.New(t)

		// Find the methods for core.Resource
		var t2 reflect.Type = reflect.TypeOf((*core.Resource)(nil)).Elem()
		coreResourceRef := TypeRef{
			pkg:  t2.PkgPath(),
			name: t2.Name(),
		}
		coreTypes, err := getTypesInPackage(coreResourceRef.pkg)
		if !assert.NoError(err) {
			return
		}
		coreResourceType := coreTypes[coreResourceRef]
		if !assert.NotEmptyf(coreResourceType, `couldn't find %v!`, coreResourceRef) {
			return
		}

		// Find all structs that implement core.Resource
		resourcesTypes, err := getTypesInPackage("github.com/klothoplatform/klotho/pkg/provider/aws/...")
		if !assert.NoError(err) {
			return
		}
		for ref, methods := range resourcesTypes {
			// Ignore all interfaces, and all structs/typedefs that don't implement core.Resource
			if methods.isInterface || !methods.containsAllMethodsIn(coreResourceType) {
				continue
			}
			assert.Contains(testedTypes, ref)
		}
	})
}

// buildExpectedTsType converts a Go type to an expected TypeScript type. For example, a map[string]int would translate
// to Record<string, number>.
func buildExpectedTsType(out *strings.Builder, tp *templatesProvider, t reflect.Type) error {
	// env vars are a special case
	if t.PkgPath() == `github.com/klothoplatform/klotho/pkg/provider/aws/resources` && t.Name() == `EnvironmentVariables` {
		out.WriteString(`Record<string, pulumi.Output<string>>`)
		return nil
	}

	// ok, general cases now
	switch t.Kind() {
	case reflect.Bool:
		out.WriteString(`boolean`)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		out.WriteString(`number`)
	case reflect.String:
		out.WriteString(`string`)
	case reflect.Struct:
		res, err := tp.getTemplateForType(t.Name())
		if err != nil {
			return err
		}
		out.WriteString(res.OutputType)
	case reflect.Array, reflect.Slice:
		out.WriteString("[]")
		buildExpectedTsType(out, tp, t.Elem())
	case reflect.Map:
		out.WriteString("Record<")
		buildExpectedTsType(out, tp, t.Key())
		out.WriteString(", ")
		buildExpectedTsType(out, tp, t.Elem())
		out.WriteRune('>')
	case reflect.Pointer, reflect.Interface:
		// Interface happens when the value is inside a map, slice, or array. Basically, the reflected type is
		// interface{},instead of being the actual type. So, we basically pull the item out of the collection, and then
		// reflect on it directly.
		buildExpectedTsType(out, tp, t.Elem())
	}
	return nil
}

// getTypesInPackage finds all types within a package (which may be "..."-ed).
func getTypesInPackage(packageName string) (map[TypeRef]Methods, error) {
	config := &packages.Config{Mode: packages.LoadSyntax}
	pkgs, err := packages.Load(config, packageName)
	if err != nil {
		return nil, err
	}
	result := make(map[TypeRef]Methods)
	for _, pkg := range pkgs {
		for _, obj := range pkg.TypesInfo.Defs {
			if obj == nil {
				continue
			}
			if _, ok := obj.(*types.TypeName); !ok {
				continue
			}
			typ, ok := obj.Type().(*types.Named)
			if !ok {
				continue
			}
			key := TypeRef{
				pkg:  pkg.PkgPath,
				name: obj.Name(),
			}
			result[key] = getMethods(typ)
		}
	}
	return result, nil
}

func getMethods(t *types.Named) Methods {
	type hasMethods interface {
		NumMethods() int
		Method(int) *types.Func
	}
	result := Methods{}
	var tMethods hasMethods = t
	if underlyingInterface, ok := t.Underlying().(*types.Interface); ok {
		tMethods = underlyingInterface
		result.isInterface = true
	}
	result.signatures = make(map[string]struct{}, tMethods.NumMethods())
	for i := 0; i < tMethods.NumMethods(); i++ {
		method := tMethods.Method(i)
		result.signatures[fmt.Sprintf(`%s %s`, method.Name(), method.Type().String())] = struct{}{}
	}
	return result
}

func (m Methods) containsAllMethodsIn(other Methods) bool {
	for sig, _ := range other.signatures {
		if _, exists := m.signatures[sig]; !exists {
			return false
		}
	}
	return true
}
