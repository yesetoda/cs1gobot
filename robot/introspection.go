package robot

import (
	"fmt"
	"sort"
	"strings"
)

// RobotState is a lightweight snapshot of a robot suitable for diagnostics,
// inspectors, and interpreted helper output.
type RobotState struct {
	// X is the current avenue of the robot.
	X int
	// Y is the current street of the robot.
	Y int
	// Direction is the robot's current facing direction.
	Direction int
	// Color is the robot's configured display colour.
	Color string
	// TracePoints is the number of recorded points in the robot's trace path.
	TracePoints int
}

// WorldDimensions returns the current world size as avenues, streets.
func WorldDimensions() (int, int) {
	if CurrentWorld == nil {
		return 0, 0
	}
	return CurrentWorld.GetAvenuesAndStreets()
}

// WorldBeeperLocations returns a copy of beeper piles keyed by world point.
func WorldBeeperLocations() map[Point]int {
	if CurrentWorld == nil {
		return map[Point]int{}
	}
	_, beepers := CurrentWorld.GetSnapshot()
	return beepers
}

// WorldBeeperTotal returns the sum of all beepers currently in the world.
func WorldBeeperTotal() int {
	total := 0
	for _, count := range WorldBeeperLocations() {
		total += count
	}
	return total
}

// WorldWallCount returns the number of wall segments in the world.
func WorldWallCount() int {
	if CurrentWorld == nil {
		return 0
	}
	walls, _ := CurrentWorld.GetSnapshot()
	return len(walls)
}

// WorldWalls returns a slice copy of all wall segments in the world.
func WorldWalls() []Wall {
	if CurrentWorld == nil {
		return nil
	}
	walls, _ := CurrentWorld.GetSnapshot()
	out := make([]Wall, 0, len(walls))
	for wall := range walls {
		out = append(out, wall)
	}
	return out
}

// WorldRobotStates returns lightweight snapshots for all active robots.
func WorldRobotStates() []RobotState {
	RegistryMu.Lock()
	robots := make([]*Robot, len(Registry))
	copy(robots, Registry)
	RegistryMu.Unlock()

	states := make([]RobotState, 0, len(robots))
	for _, r := range robots {
		if r == nil {
			continue
		}
		r.mu.Lock()
		states = append(states, RobotState{
			X:           r.X,
			Y:           r.Y,
			Direction:   r.Dir,
			Color:       r.Color,
			TracePoints: len(r.TracePath),
		})
		r.mu.Unlock()
	}
	return states
}

// WorldDetails returns a text summary useful for debugging from scripts.
func WorldDetails(maxBeeperEntries int) string {
	if CurrentWorld == nil {
		return "World: (none)"
	}

	av, st := CurrentWorld.GetAvenuesAndStreets()
	walls, beepers := CurrentWorld.GetSnapshot()

	totalBeepers := 0
	points := make([]Point, 0, len(beepers))
	for pt, count := range beepers {
		totalBeepers += count
		points = append(points, pt)
	}
	sort.Slice(points, func(i, j int) bool {
		if points[i].Y == points[j].Y {
			return points[i].X < points[j].X
		}
		return points[i].Y < points[j].Y
	})

	if maxBeeperEntries <= 0 {
		maxBeeperEntries = 10
	}

	var b strings.Builder
	fmt.Fprintf(&b, "World %dx%d\n", av, st)
	fmt.Fprintf(&b, "Walls: %d\n", len(walls))
	fmt.Fprintf(&b, "Beeper piles: %d\n", len(beepers))
	fmt.Fprintf(&b, "Beepers total: %d\n", totalBeepers)
	if len(points) == 0 {
		b.WriteString("Beeper locations: (none)\n")
	} else {
		b.WriteString("Beeper locations:\n")
		for i, pt := range points {
			if i >= maxBeeperEntries {
				fmt.Fprintf(&b, "... +%d more\n", len(points)-maxBeeperEntries)
				break
			}
			fmt.Fprintf(&b, "(%d,%d): %d\n", pt.X, pt.Y, beepers[pt])
		}
	}
	fmt.Fprintf(&b, "Robots: %d\n", len(WorldRobotStates()))

	return b.String()
}
