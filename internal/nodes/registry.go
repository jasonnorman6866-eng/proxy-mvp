package nodes

import "fmt"

type ExitNode struct {
	ID         string
	Address    string
	Authorized bool
	Healthy    bool
	Disabled   bool
}

func (n ExitNode) Available() bool {
	return n.Authorized && n.Healthy && !n.Disabled
}

type Registry struct {
	nodes map[string]ExitNode
}

func NewRegistry(init []ExitNode) *Registry {
	r := &Registry{nodes: make(map[string]ExitNode)}
	for _, n := range init {
		r.nodes[n.ID] = n
	}
	return r
}

func (r *Registry) Register(n ExitNode) {
	r.nodes[n.ID] = n
}

func (r *Registry) Disable(id string) error {
	n, ok := r.nodes[id]
	if !ok {
		return fmt.Errorf("node %q not found", id)
	}
	n.Disabled = true
	r.nodes[id] = n
	return nil
}

func (r *Registry) Enable(id string) error {
	n, ok := r.nodes[id]
	if !ok {
		return fmt.Errorf("node %q not found", id)
	}
	n.Disabled = false
	r.nodes[id] = n
	return nil
}

func (r *Registry) GetAvailable() (*ExitNode, error) {
	for _, n := range r.nodes {
		if n.Available() {
			copy := n
			return &copy, nil
		}
	}
	return nil, fmt.Errorf("no available exit node")
}
