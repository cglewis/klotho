package types

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/klothoplatform/klotho/pkg/async"
	"github.com/klothoplatform/klotho/pkg/construct"
	"github.com/klothoplatform/klotho/pkg/filter/predicate"
	"github.com/klothoplatform/klotho/pkg/io"
	"go.uber.org/zap"

	"github.com/klothoplatform/klotho/pkg/annotation"
)

type (
	ExecutionUnit struct {
		Name                 string
		files                async.ConcurrentMap[string, io.File]
		Executable           Executable
		EnvironmentVariables EnvironmentVariables
		DockerfilePath       string
		Port                 int
	}

	// Executable represents the slice of a project that is deployed to and executed on an ExecutionUnit
	Executable struct {
		Type ExecutableType

		// Entrypoints is a set of paths to source files that act as entrypoints to the Executable
		// These entrypoints are used by execunit.SourceFilesResolver as root nodes in the Executable's
		// dependency tree when resolving its set of SourceFiles.
		Entrypoints map[string]struct{}

		// Resources is a set of paths to files in the Executable's owning ExecutionUnit that represent
		// non source files required by the Executable (e.g. NodeJS's package.json file).
		Resources map[string]struct{}

		// SourceFiles is a set of paths to files in the Executable's owning ExecutionUnit that represent
		// source files that compose the Executable's application code
		SourceFiles map[string]struct{}

		// SourceFiles is a set of paths to files in the Executable's owning ExecutionUnit that represent
		// files that have been statically included for runtime use rather than active processing by the compiler.
		StaticAssets map[string]struct{}
	}

	ExecutableType string
)

func ListAllConstructs() []construct.Construct {
	return []construct.Construct{
		&ExecutionUnit{},
		&Gateway{},
		&StaticUnit{},
		&Orm{},
		&PubSub{},
		&Secrets{},
		&Kv{},
		&Fs{},
		&Config{},
		&RedisCluster{},
		&RedisNode{},
	}
}

func NewExecutable() Executable {
	return Executable{
		Entrypoints:  map[string]struct{}{},
		Resources:    map[string]struct{}{},
		StaticAssets: map[string]struct{}{},
		SourceFiles:  map[string]struct{}{},
	}
}

const EXECUTION_UNIT_TYPE = "execution_unit"

var (
	ExecutableTypeNodeJS = ExecutableType("NodeJS")
	ExecutableTypePython = ExecutableType("Python")
	ExecutableTypeGolang = ExecutableType("Golang")
	ExecutableTypeCSharp = ExecutableType("CSharp")
)

func (p *ExecutionUnit) Id() construct.ResourceId {
	return construct.ResourceId{
		Provider: construct.AbstractConstructProvider,
		Type:     EXECUTION_UNIT_TYPE,
		Name:     p.Name,
	}
}
func (p *ExecutionUnit) AnnotationCapability() string {
	return annotation.ExecutionUnitCapability
}

func (p *ExecutionUnit) Functionality() construct.Functionality {
	return construct.Compute
}

func (p *ExecutionUnit) Attributes() map[string]any {
	return map[string]any{}
}

func (unit *ExecutionUnit) OutputTo(dest string) error {
	errs := make(chan error)
	files := unit.Files()
	for idx := range files {
		go func(f io.File) {
			path := filepath.Join(dest, unit.Name, f.Path())
			dir := filepath.Dir(path)
			err := os.MkdirAll(dir, 0777)
			if err != nil {
				errs <- err
				return
			}
			file, err := os.OpenFile(path, os.O_RDWR, 0777)
			if os.IsNotExist(err) {
				file, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0777)
			} else if err == nil {
				ovr, ok := f.(io.NonOverwritable)
				if ok && !ovr.Overwrite(file) {
					errs <- nil
					return
				}
				err = file.Truncate(0)
			}
			if err != nil {
				errs <- err
				return
			}
			_, err = f.WriteTo(file)
			file.Close()
			errs <- err
		}(files[idx])
	}

	for i := 0; i < len(files); i++ {
		err := <-errs
		if err != nil {
			return err
		}
	}
	return nil
}

func (unit *ExecutionUnit) Files() map[string]io.File {
	m := make(map[string]io.File)
	for _, f := range unit.files.Values() {
		m[f.Path()] = f
	}
	return m
}

func (unit *ExecutionUnit) Add(f io.File) {
	if f != nil {
		unit.files.Set(f.Path(), f)
	}
}

func (unit *ExecutionUnit) AddResource(f io.File) {
	if f != nil {
		unit.files.Set(f.Path(), f)
		unit.Executable.Resources[f.Path()] = struct{}{}
	}
}

func (unit *ExecutionUnit) AddSourceFile(f io.File) {
	if f != nil {
		unit.files.Set(f.Path(), f)
		unit.Executable.SourceFiles[f.Path()] = struct{}{}
	}
}

func (unit *ExecutionUnit) AddStaticAsset(f io.File) {
	if f != nil {
		unit.files.Set(f.Path(), f)
		unit.Executable.StaticAssets[f.Path()] = struct{}{}
	}
}

func (unit *ExecutionUnit) AddEntrypoint(f io.File) {
	if f != nil {
		unit.files.Set(f.Path(), f)
		unit.Executable.Entrypoints[f.Path()] = struct{}{}
		unit.Executable.SourceFiles[f.Path()] = struct{}{}
	}
}

func (unit *ExecutionUnit) Remove(path string) {
	unit.Executable.RemoveFile(path)
	unit.files.Delete(path)
}

func (unit *ExecutionUnit) Get(path string) io.File {
	f, _ := unit.files.Get(path)
	return f
}

func (unit *ExecutionUnit) FilesOfLang(lang LanguageId) []*SourceFile {
	var filteredFiles []*SourceFile
	for _, file := range unit.Files() {
		if src, ok := lang.CastFile(file); ok {
			filteredFiles = append(filteredFiles, src)
		}
	}
	return filteredFiles
}

func (unit *ExecutionUnit) HasSourceFilesFor(language LanguageId) bool {
	for _, f := range unit.Files() {
		if src, isSrc := f.(*SourceFile); isSrc && src.Language.ID == language {
			return true
		}
	}
	return false
}

// GetDeclaringFiles returns a slice of files containing capability declarations for this ExecutionUnit
func (unit *ExecutionUnit) GetDeclaringFiles() []*SourceFile {
	var coreFiles []*SourceFile
	for _, f := range unit.Files() {
		astF, ok := f.(*SourceFile)
		if ok && FileExecUnitName(astF) == unit.Name {
			coreFiles = append(coreFiles, astF)
		}
	}
	return coreFiles
}

func FileExecUnitName(f *SourceFile) string {
	for _, annot := range f.Annotations() {
		cap := annot.Capability
		if cap.Name == annotation.ExecutionUnitCapability {
			if allUnits, ok := cap.Directives.Bool("all_units"); ok && allUnits {
				return ""
			}
			if cap.ID != "" {
				return cap.ID
			} else {
				return ""
			}
		}
	}
	return ""
}

func (e *Executable) RemoveFile(path string) {
	delete(e.Entrypoints, path)
	delete(e.SourceFiles, path)
	delete(e.Resources, path)
	delete(e.StaticAssets, path)
}

func InSameExecutionUnit(a, b *SourceFile) bool {
	aEU := FileExecUnitName(a)
	bEU := FileExecUnitName(b)
	res := aEU == "" || bEU == "" || aEU == bEU
	return res
}

// ContainsCapability returns whether 'a' is annotated with a capability of the supplied capability name
func ContainsCapability(a *SourceFile, capName string) bool {
	for _, cap := range a.Annotations() {
		if cap.Capability.Name == capName {
			return true
		}
	}
	return false
}

func FilePathMatchesGlob(patterns ...string) predicate.Predicate[io.File] {
	return func(target io.File) bool {
		return globMatch(target.Path(), patterns)
	}
}

func LowerCaseFilePathMatchesGlob(patterns ...string) predicate.Predicate[io.File] {
	return func(target io.File) bool {
		return globMatch(strings.ToLower(target.Path()), patterns)
	}
}

func globMatch(target string, patterns []string) bool {
	for _, pattern := range patterns {
		match, err := doublestar.Match(pattern, target)
		if err != nil {
			zap.L().Sugar().Errorf("%v", err)
			continue
		}
		if match {
			return true
		}
	}
	return false
}

func GetExecUnitForPath(fp string, cg *construct.ConstructGraph) (*ExecutionUnit, io.File) {
	var best *ExecutionUnit
	var bestFile io.File
	for _, eu := range construct.GetConstructsOfType[*ExecutionUnit](cg) {
		f := eu.Get(fp)
		if f != nil {
			astF, ok := f.(*SourceFile)
			if ok && (best == nil || FileExecUnitName(astF) == eu.Name) {
				best = eu
				bestFile = f
			}
		}
	}
	return best, bestFile
}
