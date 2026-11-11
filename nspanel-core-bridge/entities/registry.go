package entities

import (
	"sync"
)

// Registry holds the live entity instances (lights, thermostats, scenes) —
// the Go equivalent of EntityManager's static maps.
type Registry struct {
	mu          sync.RWMutex
	lights      map[int64]*Light
	thermostats map[int64]*Thermostat
	scenes      map[int64]*Scene
}

var globalRegistry *Registry
var registryOnce sync.Once

// GetRegistry returns the process-wide entity registry.
func GetRegistry() *Registry {
	registryOnce.Do(func() {
		globalRegistry = &Registry{
			lights:      make(map[int64]*Light),
			thermostats: make(map[int64]*Thermostat),
			scenes:      make(map[int64]*Scene),
		}
	})
	return globalRegistry
}

// RegisterLight adds/updates a light.
func (r *Registry) RegisterLight(l *Light) {
	r.mu.Lock()
	r.lights[l.ID] = l
	r.mu.Unlock()
}

// Light returns a light by id or nil.
func (r *Registry) Light(id int64) *Light {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lights[id]
}

// Lights returns all lights.
func (r *Registry) Lights() []*Light {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Light, 0, len(r.lights))
	for _, l := range r.lights {
		out = append(out, l)
	}
	return out
}

// RemoveLight drops a light that was deleted in the DB.
func (r *Registry) RemoveLight(id int64) {
	r.mu.Lock()
	delete(r.lights, id)
	r.mu.Unlock()
}

// RegisterThermostat adds/updates a thermostat.
func (r *Registry) RegisterThermostat(t *Thermostat) {
	r.mu.Lock()
	r.thermostats[t.ID] = t
	r.mu.Unlock()
}

// Thermostat returns a thermostat by id or nil.
func (r *Registry) Thermostat(id int64) *Thermostat {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.thermostats[id]
}

// Thermostats returns all thermostats.
func (r *Registry) Thermostats() []*Thermostat {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Thermostat, 0, len(r.thermostats))
	for _, t := range r.thermostats {
		out = append(out, t)
	}
	return out
}

// RegisterScene adds/updates a scene.
func (r *Registry) RegisterScene(s *Scene) {
	r.mu.Lock()
	r.scenes[s.ID] = s
	r.mu.Unlock()
}

// Scene returns a scene by id or nil.
func (r *Registry) Scene(id int64) *Scene {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.scenes[id]
}

// Scenes returns all scenes.
func (r *Registry) Scenes() []*Scene {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Scene, 0, len(r.scenes))
	for _, s := range r.scenes {
		out = append(out, s)
	}
	return out
}
