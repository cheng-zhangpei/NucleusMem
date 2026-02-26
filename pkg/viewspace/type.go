// pkg/viewspace/types.go

package viewspace

// ViewSpaceTree is the top-level structure holding the entire decomposed task

type NodeStatus string

const (
	NodeStatusPending  NodeStatus = "pending"
	NodeStatusBuilding NodeStatus = "building"
	NodeStatusReady    NodeStatus = "ready"
	NodeStatusRunning  NodeStatus = "running"
	NodeStatusDone     NodeStatus = "done"
	NodeStatusFailed   NodeStatus = "failed"
	NodeStatusGrowing  NodeStatus = "growing"
)

type TaskDefinition struct {
	Meta         Meta           `json:"meta"`
	ViewSpaces   []ViewSpaceDef `json:"viewspaces"`
	Dependencies Dependencies   `json:"dependencies"`
}

type Meta struct {
	TaskID      string `json:"task_id"`
	Description string `json:"description"`
}

type ViewSpaceDef struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // "global" | "process" | "atomic"
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
	Role        string   `json:"role"`
	Tools       []string `json:"tools,omitempty"` // only for atomic
}

type Dependencies struct {
	Tree     []TreeEdge     `json:"tree"`
	Dataflow []DataflowEdge `json:"dataflow"`
}

type TreeEdge struct {
	Parent   string   `json:"parent"`
	Children []string `json:"children"`
}

type DataflowEdge struct {
	From        string   `json:"from"`
	To          string   `json:"to"`
	Fields      []string `json:"fields"`
	Description string   `json:"description,omitempty"`
}

// ============================================================
// Parse errors
// ============================================================

type CheckError struct {
	Rule    string `json:"rule"`
	Node    string `json:"node,omitempty"`
	Message string `json:"message"`
}
