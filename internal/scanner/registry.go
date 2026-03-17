package scanner

import (
	"fmt"

	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

// Registry maps scanner type names to Scanner implementations.
type Registry struct {
	scanners map[string]Scanner
}

// NewRegistry returns a registry pre-populated with all built-in scanners.
func NewRegistry() *Registry {
	r := &Registry{scanners: make(map[string]Scanner)}
	r.Register(&TerraformScanner{})
	r.Register(&TerragruntScanner{})
	r.Register(&PulumiScanner{})
	r.Register(&CrossplaneScanner{})
	return r
}

// Register adds a scanner to the registry.
func (r *Registry) Register(s Scanner) {
	r.scanners[s.Type()] = s
}

// Get returns a scanner by name.
func (r *Registry) Get(name string) (Scanner, error) {
	s, ok := r.scanners[name]
	if !ok {
		return nil, fmt.Errorf("%w: unknown scanner %q", types.ErrConfigValidation, name)
	}
	return s, nil
}

// GetAll returns all scanners for the given names.
func (r *Registry) GetAll(names []string) ([]Scanner, error) {
	var result []Scanner
	for _, name := range names {
		s, err := r.Get(name)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, nil
}
