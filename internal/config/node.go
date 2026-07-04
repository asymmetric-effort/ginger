package config

// NodeKind identifies the kind of a YAML AST node.
type NodeKind int

const (
	// NodeScalar is a leaf value (string, int, float, bool, null).
	NodeScalar NodeKind = iota
	// NodeMapping is a map of key-value pairs.
	NodeMapping
	// NodeSequence is a list of values.
	NodeSequence
)

// Node is a node in the YAML AST.
type Node struct {
	Kind     NodeKind
	Value    string
	Children []*Node          // for sequences
	Keys     []string         // for mappings (preserves order)
	Map      map[string]*Node // for mappings (fast lookup)
	Line     int
	Column   int
}

// NewScalar creates a scalar node.
func NewScalar(value string, line, col int) *Node {
	return &Node{Kind: NodeScalar, Value: value, Line: line, Column: col}
}

// NewMapping creates a mapping node.
func NewMapping(line, col int) *Node {
	return &Node{Kind: NodeMapping, Map: make(map[string]*Node), Line: line, Column: col}
}

// NewSequence creates a sequence node.
func NewSequence(line, col int) *Node {
	return &Node{Kind: NodeSequence, Line: line, Column: col}
}

// SetKey sets a key-value pair in a mapping node.
func (n *Node) SetKey(key string, value *Node) {
	if _, exists := n.Map[key]; !exists {
		n.Keys = append(n.Keys, key)
	}
	n.Map[key] = value
}

// AddChild adds a child to a sequence node.
func (n *Node) AddChild(child *Node) {
	n.Children = append(n.Children, child)
}
