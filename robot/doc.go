// Package robot provides the learner-facing programming API used by Go Robots.
//
// The package models a 2D grid world made of avenues (X) and streets (Y),
// robots that move one cell at a time, beeper piles, and blocking walls. It is
// designed for teaching sequencing, conditionals, loops, decomposition, and
// concurrency with immediate visual feedback in the desktop app.
//
// Coordinates are 1-based. Avenue 1 is the leftmost column and street 1 is the
// bottom row. New robots start at (1,1) facing East unless they are created
// with NewAt.
//
// Typical programs create or load a world, create one or more robots, and then
// call movement and sensor methods such as Move, TurnLeft, FrontClear, and
// OnBeeper.
//
//	robot.CreateWorld(10, 10)
//	hubo := robot.New("blue")
//	for hubo.FrontClear() {
//		hubo.Move()
//	}
//
// The package also includes helpers for saving and loading .wld files,
// configuring animation speed, enabling traces, stepping through execution,
// and inspecting world state from demos or interpreted user code.
package robot
