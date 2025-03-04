package engine

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/klothoplatform/klotho/pkg/collectionutil"
	"github.com/klothoplatform/klotho/pkg/construct"
	"github.com/klothoplatform/klotho/pkg/engine/constraints"
	"github.com/klothoplatform/klotho/pkg/graph"
	knowledgebase "github.com/klothoplatform/klotho/pkg/knowledge_base"
	"go.uber.org/zap"
)

type (
	ExpansionSet struct {
		Construct  construct.Construct
		Attributes []string
	}

	ExpansionSolution struct {
		Graph                   *construct.ResourceGraph
		DirectlyMappedResources []construct.Resource
	}
)

// ExpandConstructs expands all constructs in the working state using the engines provider
//
// The resources that result from the expanded constructs are written to the engines resource graph
// All dependencies are copied over to the resource graph
// If a dependency in the working state included a construct, the engine copies the dependency to all directly linked resources
func (e *Engine) ExpandConstructs() {
	for _, res := range e.Context.WorkingState.ListConstructs() {
		if res.Id().Provider == construct.AbstractConstructProvider {
			zap.S().Debugf("Expanding construct %s", res.Id())
			construct, ok := res.(construct.Construct)
			if !ok {
				e.Context.Errors = append(e.Context.Errors, &ConstructExpansionError{
					Construct: res,
					Cause:     fmt.Errorf("unable to cast base construct %s to construct while expanding construct", res.Id()),
				})
				continue
			}

			constructType := ""
			attributes := make(map[string]any)
			for _, constraint := range e.Context.Constraints[constraints.ConstructConstraintScope] {
				constructConstraint, ok := constraint.(*constraints.ConstructConstraint)
				if !ok {
					e.Context.Errors = append(e.Context.Errors, &ConstructExpansionError{
						Construct: res,
						Cause:     fmt.Errorf("constraint %s is incorrect type. Expected to be a construct constraint while expanding construct", constraint),
					})
					continue
				}

				if constructConstraint.Target == construct.Id() {
					if constructType != "" && constructType != constructConstraint.Type {
						e.Context.Errors = append(e.Context.Errors, &ConstructExpansionError{
							Construct: res,
							Cause:     fmt.Errorf("unable to expand construct %s, conflicting types in constraints", res.Id()),
						})
						break
					}
					for k, v := range constructConstraint.Attributes {
						if val, ok := attributes[k]; ok {
							if v != val {
								e.Context.Errors = append(e.Context.Errors, &ConstructExpansionError{
									Construct: res,
									Cause:     fmt.Errorf("unable to expand construct %s, attribute %s has conflicting values", res.Id(), k),
								})
								break
							}
						}
						attributes[k] = v
					}
				}
			}

			for k, v := range construct.Attributes() {
				if val, ok := attributes[k]; ok {
					if v != val {
						e.Context.Errors = append(e.Context.Errors, &ConstructExpansionError{
							Construct: res,
							Cause:     fmt.Errorf("unable to expand construct %s, attribute %s has conflicting values", res.Id(), k),
						})
						break
					}
				}
				attributes[k] = v
			}
			solutions, err := e.expandConstruct(constructType, attributes, construct)
			if err != nil {
				e.Context.Errors = append(e.Context.Errors, &ConstructExpansionError{
					Construct: res,
					Cause:     err,
				})
				continue
			}
			e.Context.constructExpansionSolutions[res.Id()] = solutions
		}
	}
}

func (e *Engine) expandConstruct(constructType string, attributes map[string]any, c construct.Construct) ([]*ExpansionSolution, error) {
	var baseResource construct.Resource
	for _, res := range e.ListResources() {
		if res.Id().Type == constructType {
			baseResource = res
		}
	}
	expansionSet := ExpansionSet{Construct: c}
	for attribute := range attributes {
		expansionSet.Attributes = append(expansionSet.Attributes, attribute)
	}
	solutions, err := e.findPossibleExpansions(expansionSet, baseResource)
	if err != nil && len(solutions) == 0 {
		return nil, err
	}
	var result []*ExpansionSolution
	exists := map[string]*construct.ResourceGraph{}
	for _, sol := range solutions {
		s := sol.Graph.String()
		if exists[s] == nil {
			result = append(result, sol)
			exists[s] = sol.Graph
		}
	}
	return addNamesAndReferencesToGraphs(c, result), nil
}

func addNamesAndReferencesToGraphs(c construct.Construct, solutions []*ExpansionSolution) []*ExpansionSolution {
	endSolutions := []*ExpansionSolution{}
	for _, sol := range solutions {
		graph := construct.NewResourceGraph()
		resourceMapping := map[construct.ResourceId]construct.Resource{}
		for _, res := range sol.Graph.ListResources() {
			resval := reflect.New(reflect.TypeOf(res).Elem())
			resval.Elem().FieldByName("Name").Set(reflect.ValueOf(fmt.Sprintf("%s-%s", res.Id().Type, c.Id().Name)))
			resval.Elem().FieldByName("ConstructRefs").Set(reflect.ValueOf(construct.BaseConstructSetOf(c)))
			newResource := resval.Interface().(construct.Resource)
			resourceMapping[res.Id()] = newResource
			graph.AddResource(newResource)
		}
		for _, dep := range sol.Graph.ListDependencies() {
			graph.AddDependency(resourceMapping[dep.Source.Id()], resourceMapping[dep.Destination.Id()])
		}
		mappedRes := []construct.Resource{}
		for _, res := range sol.DirectlyMappedResources {
			mappedRes = append(mappedRes, resourceMapping[res.Id()])
		}
		endSolutions = append(endSolutions, &ExpansionSolution{Graph: graph, DirectlyMappedResources: mappedRes})
	}
	return endSolutions
}

func (e *Engine) findPossibleExpansions(expansionSet ExpansionSet, baseResource construct.Resource) ([]*ExpansionSolution, error) {
	var possibleExpansions []*ExpansionSolution
	var joinedErr error
	for _, res := range e.ListResources() {
		if baseResource != nil && res.Id().Type != baseResource.Id().Type {
			continue
		}
		classifications := e.ClassificationDocument.GetClassification(res)
		if !collectionutil.Contains(classifications.Is, string(expansionSet.Construct.Functionality())) {
			continue
		}
		unsatisfiedAttributes := []string{}
		for _, ms := range expansionSet.Attributes {
			if !collectionutil.Contains(classifications.Is, ms) {
				unsatisfiedAttributes = append(unsatisfiedAttributes, ms)
			}
		}
		graph := construct.NewResourceGraph()
		graph.AddResource(res)
		expansions, err := e.findExpansions(unsatisfiedAttributes, graph, res, expansionSet.Construct.Functionality())
		if err != nil {
			joinedErr = errors.Join(joinedErr, err)
			continue
		}
		for _, expansion := range expansions {
			possibleExpansions = append(possibleExpansions, &ExpansionSolution{Graph: expansion, DirectlyMappedResources: []construct.Resource{res}})
		}
	}
	return possibleExpansions, joinedErr
}

func (e *Engine) findExpansions(attributes []string, rg *construct.ResourceGraph, baseResource construct.Resource, functionality construct.Functionality) ([]*construct.ResourceGraph, error) {
	if len(attributes) == 0 {
		return []*construct.ResourceGraph{rg}, nil
	}
	var possibleExpansions []*construct.ResourceGraph
	for _, attribute := range attributes {
		for _, res := range e.ListResources() {
			if res.Id().Type == baseResource.Id().Type {
				continue
			}
			var paths []knowledgebase.Path
			for _, path := range e.KnowledgeBase.FindPaths(baseResource, res, knowledgebase.EdgeConstraint{}) {
				if !e.containsUnneccessaryHopsInPath(graph.Edge[construct.Resource]{Source: baseResource, Destination: res}, path, knowledgebase.EdgeData{}) {
					paths = append(paths, path)
				}
			}
			if e.ClassificationDocument.GivesAttributeForFunctionality(res, attribute, functionality) && len(paths) != 0 {
				rg.AddDependency(baseResource, res)
				unsatisfiedAttributes := []string{}
				for _, ms := range attributes {
					if ms != attribute {
						unsatisfiedAttributes = append(unsatisfiedAttributes, ms)
					}
				}

				expansions, err := e.findExpansions(unsatisfiedAttributes, rg.Clone(), baseResource, functionality)
				if err != nil {
					return nil, err
				}
				possibleExpansions = append(possibleExpansions, expansions...)
			}
		}
	}
	if len(possibleExpansions) == 0 {
		return nil, fmt.Errorf("no expansions found for attributes %v", attributes)
	}
	return possibleExpansions, nil
}
