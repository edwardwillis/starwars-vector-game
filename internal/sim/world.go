// Package sim contains the renderer-independent fixed-tick world model.
package sim

import (
	"fmt"
	"sort"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

type Command interface{ Apply(*World) error }
type SetMotion struct {
	ID     scene.ObjectID
	Motion kinematics.Motion
}

func (c SetMotion) Apply(w *World) error {
	object, ok := w.byID(c.ID)
	if !ok {
		return fmt.Errorf("object %d not found", c.ID)
	}
	object.Motion = c.Motion
	return nil
}

type Remove struct{ ID scene.ObjectID }

func (c Remove) Apply(w *World) error {
	for i, object := range w.Objects {
		if object.ID == c.ID {
			w.Objects = append(w.Objects[:i], w.Objects[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("object %d not found", c.ID)
}

type Add struct{ Object scene.Object }

func (c Add) Apply(w *World) error {
	if err := c.Object.Validate(); err != nil {
		return err
	}
	if _, ok := w.byID(c.Object.ID); ok {
		return fmt.Errorf("object %d already exists", c.Object.ID)
	}
	w.Objects = append(w.Objects, c.Object)
	sortObjects(w.Objects)
	return nil
}

type World struct {
	Tick        uint64
	Time        float64
	Objects     []scene.Object
	Frames      map[scene.FrameID]Frame
	Transitions []TransitionEvent
}

func New(objects []scene.Object) (*World, error) {
	w := &World{Objects: cloneObjects(objects), Frames: map[scene.FrameID]Frame{
		scene.ExteriorFrame: {ID: scene.ExteriorFrame, Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()}},
	}}
	for _, object := range w.Objects {
		if err := object.Validate(); err != nil {
			return nil, err
		}
	}
	sortObjects(w.Objects)
	return w, nil
}

func (w *World) Apply(commands ...Command) error {
	for _, command := range commands {
		if command == nil {
			continue
		}
		if err := command.Apply(w); err != nil {
			return err
		}
	}
	return nil
}

func (w *World) Step(seconds float64) error {
	if seconds <= 0 {
		return fmt.Errorf("simulation step must be positive")
	}
	for i := range w.Objects {
		w.Objects[i].Pose = kinematics.Integrate(w.Objects[i].Pose, w.Objects[i].Motion, seconds)
	}
	w.Tick++
	w.Time += seconds
	return nil
}

func (w *World) Snapshot() Snapshot {
	frames := make(map[scene.FrameID]Frame, len(w.Frames))
	for id, frame := range w.Frames {
		frames[id] = frame
	}
	return Snapshot{Tick: w.Tick, Time: w.Time, Objects: cloneObjects(w.Objects), Frames: frames, Transitions: append([]TransitionEvent(nil), w.Transitions...)}
}

type Snapshot struct {
	Tick        uint64
	Time        float64
	Objects     []scene.Object
	Frames      map[scene.FrameID]Frame
	Transitions []TransitionEvent
}

func (w *World) byID(id scene.ObjectID) (*scene.Object, bool) {
	for i := range w.Objects {
		if w.Objects[i].ID == id {
			return &w.Objects[i], true
		}
	}
	return nil, false
}
func cloneObjects(objects []scene.Object) []scene.Object {
	out := make([]scene.Object, len(objects))
	copy(out, objects)
	for i := range out {
		out[i].Parts = append([]scene.Part(nil), objects[i].Parts...)
		out[i].Anchors = cloneAnchors(objects[i].Anchors)
	}
	return out
}
func cloneAnchors(in map[string]kinematics.Pose) map[string]kinematics.Pose {
	if in == nil {
		return nil
	}
	out := make(map[string]kinematics.Pose, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
func sortObjects(objects []scene.Object) {
	sort.Slice(objects, func(i, j int) bool { return objects[i].ID < objects[j].ID })
}
