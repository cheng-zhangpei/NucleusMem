package viewspace

import (
	"encoding/json"
	"fmt"
)

// ============================================================
// Schema structs (what LLM outputs)
// ============================================================

func (e *CheckError) Error() string {
	if e.Node != "" {
		return fmt.Sprintf("[%s] node '%s': %s", e.Rule, e.Node, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Rule, e.Message)
}

type ParseResult struct {
	Definition *TaskDefinition
	Errors     []*CheckError
	Warnings   []string
}

// ============================================================
// Parse: JSON decode + check
// ============================================================

func Parse(raw []byte) *ParseResult {
	result := &ParseResult{}

	// Phase 1: JSON decode
	var def TaskDefinition
	if err := json.Unmarshal(raw, &def); err != nil {
		result.Errors = append(result.Errors, &CheckError{
			Rule:    "JSON",
			Message: fmt.Sprintf("invalid JSON: %v", err),
		})
		return result
	}
	result.Definition = &def

	// Phase 2: structural checks
	checkers := []func(*TaskDefinition) *CheckError{
		checkHasMetaTaskID,
		checkUniqueGlobal,
		checkNamesUnique,
		checkAtomicNoChildren,
		checkNonLeafHasChildren,
		checkReferencesExist,
		checkTreeConnected,
		checkDataflowAcyclic,
		checkAtomicHasTools,
		checkProcessNoTools,
	}

	for _, check := range checkers {
		if err := check(&def); err != nil {
			result.Errors = append(result.Errors, err)
		}
	}

	// Phase 3: warnings
	result.Warnings = checkWarnings(&def)

	return result
}

// HasErrors returns true if any hard rule failed
func (r *ParseResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// FormatErrorsForLLM formats check errors into a string that can be fed back to LLM
func (r *ParseResult) FormatErrorsForLLM() string {
	if !r.HasErrors() {
		return ""
	}
	msg := "Your ViewSpace Tree has the following errors. Please fix them and output the corrected JSON:\n\n"
	for i, err := range r.Errors {
		msg += fmt.Sprintf("%d. [%s] %s", i+1, err.Rule, err.Message)
		if err.Node != "" {
			msg += fmt.Sprintf(" (node: '%s')", err.Node)
		}
		msg += "\n"
	}
	return msg
}

// ============================================================
// Hard rules
// ============================================================

func checkHasMetaTaskID(def *TaskDefinition) *CheckError {
	if def.Meta.TaskID == "" {
		return &CheckError{Rule: "META", Message: "meta.task_id is required"}
	}
	return nil
}

// H1: exactly one global
func checkUniqueGlobal(def *TaskDefinition) *CheckError {
	count := 0
	for _, vs := range def.ViewSpaces {
		if vs.Type == "global" {
			count++
		}
	}
	if count == 0 {
		return &CheckError{Rule: "H1", Message: "no global viewspace found, exactly 1 required"}
	}
	if count > 1 {
		return &CheckError{Rule: "H1", Message: fmt.Sprintf("found %d global viewspaces, exactly 1 required", count)}
	}
	return nil
}

// H3: all names unique
func checkNamesUnique(def *TaskDefinition) *CheckError {
	seen := map[string]bool{}
	for _, vs := range def.ViewSpaces {
		if vs.Name == "" {
			return &CheckError{Rule: "H3", Message: "viewspace has empty name"}
		}
		if seen[vs.Name] {
			return &CheckError{Rule: "H3", Node: vs.Name, Message: "duplicate name"}
		}
		seen[vs.Name] = true
	}
	return nil
}

// H2: atomic cannot be parent in tree
func checkAtomicNoChildren(def *TaskDefinition) *CheckError {
	atomics := map[string]bool{}
	for _, vs := range def.ViewSpaces {
		if vs.Type == "atomic" {
			atomics[vs.Name] = true
		}
	}
	for _, edge := range def.Dependencies.Tree {
		if atomics[edge.Parent] {
			return &CheckError{Rule: "H2", Node: edge.Parent, Message: "atomic viewspace cannot have children"}
		}
	}
	return nil
}

// H3b: global and process must have children
func checkNonLeafHasChildren(def *TaskDefinition) *CheckError {
	// Build set of nodes that appear as parents
	parents := map[string]bool{}
	for _, edge := range def.Dependencies.Tree {
		parents[edge.Parent] = true
	}

	for _, vs := range def.ViewSpaces {
		if vs.Type == "global" || vs.Type == "process" {
			if !parents[vs.Name] {
				return &CheckError{
					Rule:    "H3",
					Node:    vs.Name,
					Message: fmt.Sprintf("%s viewspace must have at least one child in tree", vs.Type),
				}
			}
		}
	}
	return nil
}

// H8: all names in tree and dataflow must exist in viewspaces
func checkReferencesExist(def *TaskDefinition) *CheckError {
	known := map[string]bool{}
	for _, vs := range def.ViewSpaces {
		known[vs.Name] = true
	}

	for _, edge := range def.Dependencies.Tree {
		if !known[edge.Parent] {
			return &CheckError{Rule: "H8", Node: edge.Parent, Message: "referenced in tree but not defined in viewspaces"}
		}
		for _, child := range edge.Children {
			if !known[child] {
				return &CheckError{Rule: "H8", Node: child, Message: "referenced in tree but not defined in viewspaces"}
			}
		}
	}

	for _, edge := range def.Dependencies.Dataflow {
		if !known[edge.From] {
			return &CheckError{Rule: "H8", Node: edge.From, Message: "referenced in dataflow but not defined in viewspaces"}
		}
		if !known[edge.To] {
			return &CheckError{Rule: "H8", Node: edge.To, Message: "referenced in dataflow but not defined in viewspaces"}
		}
	}
	return nil
}

// H4: tree must form a connected tree rooted at global
func checkTreeConnected(def *TaskDefinition) *CheckError {
	if len(def.ViewSpaces) <= 1 {
		return nil // single node is trivially connected
	}

	// Find global node
	globalName := ""
	for _, vs := range def.ViewSpaces {
		if vs.Type == "global" {
			globalName = vs.Name
			break
		}
	}
	if globalName == "" {
		return nil // H1 will catch this
	}

	// Build children map
	children := map[string][]string{}
	for _, edge := range def.Dependencies.Tree {
		children[edge.Parent] = append(children[edge.Parent], edge.Children...)
	}

	// BFS from global
	visited := map[string]bool{}
	queue := []string{globalName}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if visited[node] {
			continue
		}
		visited[node] = true
		for _, child := range children[node] {
			queue = append(queue, child)
		}
	}

	// Check all viewspaces are reachable
	for _, vs := range def.ViewSpaces {
		if !visited[vs.Name] {
			return &CheckError{
				Rule:    "H4",
				Node:    vs.Name,
				Message: "not reachable from global node in tree, tree is disconnected",
			}
		}
	}
	return nil
}

// H5: dataflow must be acyclic
func checkDataflowAcyclic(def *TaskDefinition) *CheckError {
	inDegree := map[string]int{}
	adj := map[string][]string{}

	for _, vs := range def.ViewSpaces {
		inDegree[vs.Name] = 0
	}
	for _, edge := range def.Dependencies.Dataflow {
		adj[edge.From] = append(adj[edge.From], edge.To)
		inDegree[edge.To]++
	}

	queue := []string{}
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++
		for _, next := range adj[node] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if visited != len(inDegree) {
		return &CheckError{Rule: "H5", Message: "circular dependency detected in dataflow"}
	}
	return nil
}

// Atomic should have tools
func checkAtomicHasTools(def *TaskDefinition) *CheckError {
	for _, vs := range def.ViewSpaces {
		if vs.Type == "atomic" && len(vs.Tools) == 0 {
			return &CheckError{
				Rule:    "TOOL",
				Node:    vs.Name,
				Message: "atomic viewspace should have at least one tool",
			}
		}
	}
	return nil
}

// Process/Global should not have tools
func checkProcessNoTools(def *TaskDefinition) *CheckError {
	for _, vs := range def.ViewSpaces {
		if (vs.Type == "global" || vs.Type == "process") && len(vs.Tools) > 0 {
			return &CheckError{
				Rule:    "TOOL",
				Node:    vs.Name,
				Message: fmt.Sprintf("%s viewspace should not have tools, only atomic nodes have tools", vs.Type),
			}
		}
	}
	return nil
}

// ============================================================
// Soft rules (warnings)
// ============================================================

func checkWarnings(def *TaskDefinition) []string {
	var warnings []string

	// Tree depth check
	depth := calcMaxDepth(def)
	if depth > 4 {
		warnings = append(warnings, fmt.Sprintf("S2: tree depth is %d, consider simplifying (>4 may cause latency)", depth))
	}

	// Too many children
	for _, edge := range def.Dependencies.Tree {
		if len(edge.Children) > 10 {
			warnings = append(warnings, fmt.Sprintf("S3: '%s' has %d children, consider grouping into sub-processes", edge.Parent, len(edge.Children)))
		}
	}

	return warnings
}

func calcMaxDepth(def *TaskDefinition) int {
	children := map[string][]string{}
	for _, edge := range def.Dependencies.Tree {
		children[edge.Parent] = append(children[edge.Parent], edge.Children...)
	}

	globalName := ""
	for _, vs := range def.ViewSpaces {
		if vs.Type == "global" {
			globalName = vs.Name
			break
		}
	}
	if globalName == "" {
		return 0
	}

	var dfs func(string) int
	dfs = func(node string) int {
		maxChild := 0
		for _, child := range children[node] {
			d := dfs(child)
			if d > maxChild {
				maxChild = d
			}
		}
		return maxChild + 1
	}
	return dfs(globalName)
}
func ExtractJSON(response string) string {
	// Try direct parse first
	var js json.RawMessage
	if json.Unmarshal([]byte(response), &js) == nil {
		return response
	}

	// Try to find JSON between ```json and ```
	start := -1
	for i := 0; i < len(response)-6; i++ {
		if i+7 <= len(response) && response[i:i+7] == "```json" {
			start = i + 7
			break
		}
	}
	// Also try plain ``` block
	if start == -1 {
		for i := 0; i < len(response)-2; i++ {
			if response[i:i+3] == "```" {
				start = i + 3
				break
			}
		}
	}

	if start >= 0 {
		for i := start; i < len(response)-2; i++ {
			if response[i:i+3] == "```" {
				candidate := response[start:i]
				for len(candidate) > 0 && (candidate[0] == '\n' || candidate[0] == ' ') {
					candidate = candidate[1:]
				}
				if json.Unmarshal([]byte(candidate), &js) == nil {
					return candidate
				}
			}
		}
	}

	// Try to find first { and last }
	firstBrace := -1
	lastBrace := -1
	for i, c := range response {
		if c == '{' && firstBrace == -1 {
			firstBrace = i
		}
		if c == '}' {
			lastBrace = i
		}
	}

	if firstBrace >= 0 && lastBrace > firstBrace {
		candidate := response[firstBrace : lastBrace+1]
		if json.Unmarshal([]byte(candidate), &js) == nil {
			return candidate
		}
	}

	return ""
}
