package engine

import (
	"fmt"
	"math"
	"reflect"
	"sort"

	"github.com/klothoplatform/klotho/pkg/collectionutil"
	"github.com/klothoplatform/klotho/pkg/construct"
	"github.com/klothoplatform/klotho/pkg/graph"
	knowledgebase "github.com/klothoplatform/klotho/pkg/knowledge_base"
	"go.uber.org/zap"
)

func (e *Engine) expandEdge(dep graph.Edge[construct.Resource], context *SolveContext) EngineError {
	graph := context.ResourceGraph
	edgeData, err := getEdgeData(dep)
	if err != nil {
		return &EdgeExpansionError{
			Edge:  dep,
			Cause: err,
		}
	}
	// if its a direct edge and theres no constraint on what needs to exist then we should be able to just return
	if _, found := e.KnowledgeBase.GetResourceEdge(dep.Source, dep.Destination); len(edgeData.Constraint.NodeMustExist) == 0 && found {
		return nil
	}
	paths, err := e.determineCorrectPaths(dep, edgeData)
	if err != nil {
		zap.S().Warnf("got error when determining correct path for edge %s -> %s, err: %s", dep.Source.Id(), dep.Destination.Id(), err.Error())
		return &EdgeExpansionError{
			Edge:  dep,
			Cause: err,
		}
	}
	if len(paths) == 0 {
		return &EdgeExpansionError{
			Edge:  dep,
			Cause: fmt.Errorf("no paths found that satisfy the attributes, %s, and do not contain unnecessary hops for edge %s -> %s", edgeData.Attributes, dep.Source.Id(), dep.Destination.Id()),
		}
	}
	path := e.findOptimalPath(paths)
	if len(path) == 0 {
		return &InternalError{
			Child: &EdgeExpansionError{Edge: dep},
			Cause: fmt.Errorf("empty path found that satisfy the attributes, %s, and do not contain unnecessary hops for edge %s -> %s", edgeData.Attributes, dep.Source.Id(), dep.Destination.Id()),
		}
	}
	edges := e.KnowledgeBase.ExpandEdge(&dep, graph, path, edgeData)
	if len(edges) > 1 {
		zap.S().Debugf("Removing dependency from %s -> %s", dep.Source.Id(), dep.Destination.Id())
		err := graph.RemoveDependency(dep.Source.Id(), dep.Destination.Id())
		if err != nil {
			return &EdgeExpansionError{
				Cause: fmt.Errorf("error removing dependency %s -> %s", dep.Source.Id(), dep.Destination.Id()),
				Edge:  dep,
			}
		}
	}
	for _, edge := range edges {
		zap.S().Debugf("Adding dependency from %s -> %s", edge.Source.Id(), edge.Destination.Id())
		e.handleDecision(context, Decision{
			Action: ActionConnect,
			Result: &DecisionResult{Edge: &edge},
			Cause:  &Cause{EdgeExpansion: &dep},
		})
	}
	return nil
}

// getEdgeData retrieves the edge data from the edge in the resource graph to use during expansion
func getEdgeData(dep graph.Edge[construct.Resource]) (knowledgebase.EdgeData, error) {
	// We want to retrieve the edge data from the edge in the resource graph to use during expansion
	edgeData := knowledgebase.EdgeData{}
	data, ok := dep.Properties.Data.(knowledgebase.EdgeData)
	if !ok && dep.Properties.Data != nil {
		return edgeData, fmt.Errorf("edge properties for edge %s -> %s, do not satisfy edge data format during expansion", dep.Source.Id(), dep.Destination.Id())
	} else if dep.Properties.Data != nil {
		edgeData = data
	}
	// We attach the dependencies source and destination nodes for context during expansion
	edgeData.Source = dep.Source
	edgeData.Destination = dep.Destination
	// Find all possible paths given the initial source and destination node
	return edgeData, nil
}

// determineCorrectPath determines the correct path to take to get from the dependency's source node to destination node, using the knowledgebase of edges
// It first finds all possible paths given the initial source and destination node. It then filters out any paths that do not satisfy the constraints of the edge
// It then filters out any paths that contain unnecessary hops to get to the destination
func (e *Engine) determineCorrectPaths(dep graph.Edge[construct.Resource], edgeData knowledgebase.EdgeData) ([]knowledgebase.Path, error) {
	paths := e.KnowledgeBase.FindPaths(dep.Source, dep.Destination, edgeData.Constraint)
	var validPaths []knowledgebase.Path
	var satisfyAttributeData []knowledgebase.Path
	for _, p := range paths {
		satisfies := true
		for _, edge := range p {
			for k := range edgeData.Attributes {
				// If its a direct edge we need to make sure the source contains the attributes, otherwise ignore the source of the dependency
				if edge.Source != reflect.TypeOf(dep.Source) || len(p) == 1 {
					classification := e.ClassificationDocument.GetClassification(reflect.New(edge.Source.Elem()).Interface().(construct.Resource))
					if !collectionutil.Contains(classification.Is, k) {
						satisfies = false
						break
					}
				}
				// If its a direct edge we need to make sure the destination contains the attributes, otherwise ignore the destination of the dependency
				if edge.Destination != reflect.TypeOf(dep.Destination) || len(p) == 1 {
					classification := e.ClassificationDocument.GetClassification(reflect.New(edge.Destination.Elem()).Interface().(construct.Resource))
					if !collectionutil.Contains(classification.Is, k) {
						satisfies = false
						break
					}
				}
			}
			if !satisfies {
				break
			}
		}
		if satisfies {
			satisfyAttributeData = append(satisfyAttributeData, p)
		}
	}
	if len(satisfyAttributeData) == 0 {
		return nil, fmt.Errorf("no paths found that satisfy the attributes, %s, for edge %s -> %s", edgeData.Attributes, dep.Source.Id(), dep.Destination.Id())
	}
	for _, p := range satisfyAttributeData {
		// Ensure we arent taking unnecessary hops to get to the destination
		if !e.containsUnneccessaryHopsInPath(dep, p, edgeData) {
			validPaths = append(validPaths, p)
		}
	}
	return validPaths, nil
}

// containsUnneccessaryHopsInPath determines if the path contains any unnecessary hops to get to the destination
//
// We check if the source and destination of the dependency have a functionality. If they do, we check if the functionality of the source or destination
// is the same as the functionality of the source or destination of the edge in the path. If it is then we ensure that the source or destination of the edge
// in the path is not the same as the source or destination of the dependency. If it is then we know that the edge in the path is an unnecessary hop to get to the destination
func (e *Engine) containsUnneccessaryHopsInPath(dep graph.Edge[construct.Resource], p knowledgebase.Path, edgeData knowledgebase.EdgeData) bool {
	destType := reflect.TypeOf(dep.Destination)
	srcType := reflect.TypeOf(dep.Source)

	var mustExistTypes []reflect.Type
	for _, res := range edgeData.Constraint.NodeMustExist {
		mustExistTypes = append(mustExistTypes, reflect.TypeOf(res))
	}

	// Here we check if the edge or destination functionality exist within the path in another resource. If they do, we know that the path contains unnecessary hops.
	for _, edge := range p {
		if e.ClassificationDocument.GetFunctionality(dep.Destination) != construct.Unknown {
			if e.ClassificationDocument.GetFunctionality(dep.Destination) == e.ClassificationDocument.GetFunctionality(reflect.New(edge.Destination).Elem().Interface().(construct.Resource)) && edge.Destination != destType && edge.Destination != srcType &&
				!collectionutil.Contains(mustExistTypes, edge.Destination) {
				return true
			}
			if e.ClassificationDocument.GetFunctionality(dep.Destination) == e.ClassificationDocument.GetFunctionality(reflect.New(edge.Source).Elem().Interface().(construct.Resource)) && edge.Source != destType && edge.Source != srcType &&
				!collectionutil.Contains(mustExistTypes, edge.Source) {
				return true
			}
		}
		if e.ClassificationDocument.GetFunctionality(dep.Source) != construct.Unknown {
			if e.ClassificationDocument.GetFunctionality(dep.Source) == e.ClassificationDocument.GetFunctionality(reflect.New(edge.Destination).Elem().Interface().(construct.Resource)) && edge.Destination != srcType && edge.Destination != destType &&
				!collectionutil.Contains(mustExistTypes, edge.Destination) {
				return true
			}
			if e.ClassificationDocument.GetFunctionality(dep.Source) == e.ClassificationDocument.GetFunctionality(reflect.New(edge.Source).Elem().Interface().(construct.Resource)) && edge.Source != srcType && edge.Source != destType &&
				!collectionutil.Contains(mustExistTypes, edge.Source) {
				return true
			}
		}
	}

	// Now we will look to see if there are duplicate functionality in resources within the edge, if there are we will say it contains unnecessary hops. We will verify first that those duplicates dont exist because of a constraint
	foundFunc := map[construct.Functionality]bool{}
	for _, res := range edgeData.Constraint.NodeMustExist {
		mustExistTypes = append(mustExistTypes, reflect.TypeOf(res))
	}
	for _, edge := range p {
		if edge.Source != srcType && !collectionutil.Contains(mustExistTypes, edge.Source) {
			functionality := e.ClassificationDocument.GetFunctionality(reflect.New(edge.Source).Elem().Interface().(construct.Resource))
			if foundFunc[functionality] && functionality != construct.Unknown {
				return true
			}
			foundFunc[functionality] = true
		}
	}

	return false
}

func (e *Engine) findOptimalPath(paths []knowledgebase.Path) knowledgebase.Path {
	lowestWeightPaths := e.findLowestWeightPaths(paths)
	return findShortestPath(lowestWeightPaths)
}

// findShortestPath determines the shortest path to get from the dependency's source node to destination node, using the knowledgebase of edges
func findShortestPath(paths []knowledgebase.Path) knowledgebase.Path {
	var validPath []knowledgebase.Edge

	var sameLengthPaths []knowledgebase.Path
	// Get the shortest route that satisfied constraints
	for _, path := range paths {
		if len(validPath) == 0 {
			validPath = path
		} else if len(path) < len(validPath) {
			validPath = path
			sameLengthPaths = []knowledgebase.Path{}
		} else if len(path) == len(validPath) {
			sameLengthPaths = append(sameLengthPaths, path, validPath)
		}
	}
	// If there are multiple paths with the same length we are going to generate a string for each and sort them so we can be deterministic in which one we choose
	if len(sameLengthPaths) > 0 {
		pathStrings := map[string]knowledgebase.Path{}
		for _, p := range sameLengthPaths {
			pString := ""
			for _, r := range p {
				pString += fmt.Sprintf("%s -> %s", r.Source, r.Destination)
			}
			pathStrings[pString] = p
		}
		keys := make([]string, 0, len(pathStrings))
		for k := range pathStrings {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return pathStrings[keys[0]]
	}
	return validPath
}

// findLowestWeightPaths ranks the paths based on the weight of the path and returns the ranked paths (lowest weight first)
func (e *Engine) findLowestWeightPaths(paths []knowledgebase.Path) []knowledgebase.Path {
	lowestWeight := math.MaxInt
	var lowestWeightPaths []knowledgebase.Path
	for _, path := range paths {
		weight := e.resolvePathWeight(path)
		if weight < lowestWeight {
			lowestWeight = weight
			lowestWeightPaths = []knowledgebase.Path{path}
		} else if weight == lowestWeight {
			lowestWeightPaths = append(lowestWeightPaths, path)
		}
	}

	return lowestWeightPaths
}

func (e *Engine) resolvePathWeight(path knowledgebase.Path) int {
	weight := 0
	for _, edge := range path {
		rEdge := toResourceEdge(edge)
		weight += e.resolveEdgeWeight(rEdge)
	}
	return weight
}

func (e *Engine) resolveEdgeWeight(edge graph.Edge[construct.Resource]) int {
	weight := 0
	if e.ClassificationDocument.GetFunctionality(edge.Source) != construct.Unknown {
		weight += 1
	}
	if e.ClassificationDocument.GetFunctionality(edge.Destination) != construct.Unknown {
		weight += 1
	}
	return weight
}

func toResourceEdge(edge knowledgebase.Edge) graph.Edge[construct.Resource] {
	src := reflect.New(edge.Source.Elem()).Interface().(construct.Resource)
	dest := reflect.New(edge.Destination.Elem()).Interface().(construct.Resource)
	return graph.Edge[construct.Resource]{Source: src, Destination: dest}
}
