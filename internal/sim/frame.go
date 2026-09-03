package sim

import (
	"fmt"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

// Frame is a coordinate space optionally attached to a host world object.
type Frame struct {
	ID          scene.FrameID
	HostID      scene.ObjectID
	Environment string
	Pose        kinematics.Pose
}

type TransitionEvent struct {
	Tick        uint64
	ObjectID    scene.ObjectID
	HostID      scene.ObjectID
	Environment string
	Anchor      string
	Source      scene.FrameID
	Destination scene.FrameID
}

type RegisterFrame struct{ Frame Frame }

func (command RegisterFrame) Apply(world *World) error {
	frame := command.Frame
	if frame.ID == "" || frame.ID == scene.ExteriorFrame {
		return fmt.Errorf("custom frame ID is required")
	}
	if _, exists := world.Frames[frame.ID]; exists {
		return fmt.Errorf("frame %q already exists", frame.ID)
	}
	if frame.HostID != 0 {
		if _, ok := world.byID(frame.HostID); !ok {
			return fmt.Errorf("frame %q host object %d not found", frame.ID, frame.HostID)
		}
	}
	world.Frames[frame.ID] = frame
	return nil
}

// Transfer moves an object between frames without changing its exterior pose.
type Transfer struct {
	ObjectID    scene.ObjectID
	Destination scene.FrameID
	Anchor      string
}

func (command Transfer) Apply(world *World) error {
	object, ok := world.byID(command.ObjectID)
	if !ok {
		return fmt.Errorf("object %d not found", command.ObjectID)
	}
	destination := command.Destination
	if destination == "" {
		destination = scene.ExteriorFrame
	}
	if _, ok := world.Frames[destination]; !ok {
		return fmt.Errorf("destination frame %q not found", destination)
	}
	source := normalizedFrame(object.Frame)
	if source == destination {
		return nil
	}
	worldPose, err := world.objectWorldPose(*object, make(map[scene.FrameID]bool))
	if err != nil {
		return err
	}
	destinationPose, err := world.frameWorldPose(destination, make(map[scene.FrameID]bool))
	if err != nil {
		return err
	}
	sourcePose, err := world.frameWorldPose(source, make(map[scene.FrameID]bool))
	if err != nil {
		return err
	}
	worldVelocity := sourcePose.Orientation.Rotate(object.Motion.Velocity)
	object.Pose = kinematics.Relative(destinationPose, worldPose)
	object.Motion.Velocity = destinationPose.Orientation.Conjugate().Rotate(worldVelocity)
	object.Frame = destination
	eventFrame := world.Frames[destination]
	if destination == scene.ExteriorFrame {
		eventFrame = world.Frames[source]
	}
	world.Transitions = append(world.Transitions, TransitionEvent{
		Tick:        world.Tick,
		ObjectID:    object.ID,
		HostID:      eventFrame.HostID,
		Environment: eventFrame.Environment,
		Anchor:      command.Anchor,
		Source:      source,
		Destination: destination,
	})
	return nil
}

func normalizedFrame(id scene.FrameID) scene.FrameID {
	if id == "" {
		return scene.ExteriorFrame
	}
	return id
}

func (world *World) WorldPose(id scene.ObjectID) (kinematics.Pose, error) {
	object, ok := world.byID(id)
	if !ok {
		return kinematics.Pose{}, fmt.Errorf("object %d not found", id)
	}
	return world.objectWorldPose(*object, make(map[scene.FrameID]bool))
}

// FramePose resolves a registered frame into exterior/world coordinates.
func (world *World) FramePose(frameID scene.FrameID) (kinematics.Pose, error) {
	return world.frameWorldPose(frameID, make(map[scene.FrameID]bool))
}

// PoseInFrame resolves an object's pose into another registered coordinate
// frame without transferring it.
func (world *World) PoseInFrame(id scene.ObjectID, frameID scene.FrameID) (kinematics.Pose, error) {
	worldPose, err := world.WorldPose(id)
	if err != nil {
		return kinematics.Pose{}, err
	}
	framePose, err := world.FramePose(frameID)
	if err != nil {
		return kinematics.Pose{}, err
	}
	return kinematics.Relative(framePose, worldPose), nil
}

func (world *World) frameWorldPose(id scene.FrameID, visiting map[scene.FrameID]bool) (kinematics.Pose, error) {
	id = normalizedFrame(id)
	frame, ok := world.Frames[id]
	if !ok {
		return kinematics.Pose{}, fmt.Errorf("frame %q not found", id)
	}
	if frame.HostID == 0 {
		return frame.Pose, nil
	}
	if visiting[id] {
		return kinematics.Pose{}, fmt.Errorf("frame cycle at %q", id)
	}
	visiting[id] = true
	host, ok := world.byID(frame.HostID)
	if !ok {
		return kinematics.Pose{}, fmt.Errorf("frame %q host object %d not found", id, frame.HostID)
	}
	hostPose, err := world.objectWorldPose(*host, visiting)
	delete(visiting, id)
	if err != nil {
		return kinematics.Pose{}, err
	}
	return kinematics.Compose(hostPose, frame.Pose), nil
}

func (world *World) objectWorldPose(object scene.Object, visiting map[scene.FrameID]bool) (kinematics.Pose, error) {
	framePose, err := world.frameWorldPose(normalizedFrame(object.Frame), visiting)
	if err != nil {
		return kinematics.Pose{}, err
	}
	return kinematics.Compose(framePose, object.Pose), nil
}
