# Go Robots Guide

This guide explains how to use and extend the Go Robots project in depth.
It is intended for learners, instructors, and contributors.

## 1. Mental Model

Go Robots uses a grid world:

- `X` is the avenue (left to right), starting at `1`.
- `Y` is the street (bottom to top), starting at `1`.
- A world has bounded dimensions (`avenues`, `streets`).
- Walls block movement between adjacent cells.
- Beepers are integer counts on cells.

By default, `robot.New(...)` creates a robot at `(1,1)` facing `East`.

## 2. Starting The App

From the repository root:

```bash
go run .
```

Main surfaces:

- Left: rendered world grid.
- Right top: controls (world setup, simulation, debugger, edit mode, demos).
- Right bottom: code editor with `Run Code`.

Alternative mode:

```bash
go run ./cmd/hubo_demo
```

Compiled demo mode keeps the UI but disables editor execution.

## 3. Writing Programs In The Built-In Editor

### Minimum skeleton

```go
package main

import "github.com/yesetoda/cs1gobot/robot"

func main() {
    hubo := robot.New("blue")
    hubo.Move()
}
```

### Interpreter behavior (Yaegi)

`engine/yaegi.go` normalizes source before execution:

- If no package is declared, it inserts `package main`.
- If another package name is used, it is rewritten to `main`.

This makes short snippets easier to run in the editor.

### Runtime error behavior

Robot invalid actions use `panic` internally (for example moving into a wall).
The engine recovers panics and surfaces them as regular errors in the UI.

## 4. Robot API Reference

The following API is exposed to interpreted code through the Yaegi symbol table.

### Package-level functions

| Function | Description |
|---|---|
| `robot.New(color ...interface{})` | Create robot at `(1,1)`, facing east. Optional first arg color string. |
| `robot.NewAt(av, st, dir, color)` | Create robot at specific position/direction. |
| `robot.CreateWorld(avenues, streets)` | Replace current world with new empty world. `0` values fall back to `10`. |
| `robot.LoadWorld(path)` | Load world from `.wld` file. |
| `robot.SaveWorld(path)` | Save current world to `.wld` file. |
| `robot.Reset()` | Remove all registered robots and reset step counters. |
| `robot.SetDefaultDelay(ms)` | Delay for newly created robots only. |
| `robot.SetDefaultTraceColor(color)` | Non-empty enables default tracing for new robots. Empty disables. |
| `robot.SetStop(bool)` | Stop execution flag used by runner/UI controls. |
| `robot.WorldDimensions()` | Returns current world size. |
| `robot.WorldBeeperLocations()` | Returns map of beeper piles by point. |
| `robot.WorldBeeperTotal()` | Total beeper count in world. |
| `robot.WorldWallCount()` | Number of wall segments. |
| `robot.WorldWalls()` | Copy of all walls. |
| `robot.WorldRobotStates()` | Snapshot of current robot states. |
| `robot.WorldDetails(maxEntries)` | Human-readable world summary string. |

### Direction constants

- `robot.North`
- `robot.West`
- `robot.South`
- `robot.East`

### Robot methods

| Method | Description |
|---|---|
| `Move()` | Move one cell forward. Panics if blocked by wall or border. |
| `TurnLeft()` | Rotate 90 degrees left. |
| `TurnRight()` | Rotate 90 degrees right. |
| `PickBeeper()` | Pick one beeper from current cell. Panics if none exists. |
| `DropBeeper()` | Drop one beeper on current cell. Panics if bag empty. |
| `SetDelay(ms)` | Per-action delay for this robot. |
| `SetPause(ms)` | Alias of `SetDelay`. |
| `SetTrace(colorString)` | Enable trace when non-empty, disable when empty. |
| `FrontClear()` / `FrontIsClear()` | Sensor: path in front is clear. |
| `LeftClear()` / `LeftIsClear()` | Sensor: path to left is clear. |
| `RightClear()` / `RightIsClear()` | Sensor: path to right is clear. |
| `OnBeeper()` | Sensor: current cell has beeper(s). |
| `CarriesBeepers()` | Sensor: robot has beepers in bag. |
| `FacingNorth()` | Sensor: robot currently faces north. |
| `GetState()` | Returns `(x, y, dir, color)`. |
| `GetTrace()` | Returns copy of trace points. |

## 5. UI Controls In Detail

Defined in `ui/window.go` and `ui/grid.go`.

### World Setup

- `World...` selector creates empty presets:
- `Empty 10x10`
- `Empty 12x8`
- `Empty 20x10`

- `Prebuilt world...` loads `.wld` files found in:
- `worlds/`
- `go_robots/worlds` (fallback path)

- `Refresh prebuilt list` rescans world files.
- `Load .wld file` opens a file chooser.
- `Save world to .wld` exports current world state.

### Simulation

- `Speed (ms per step)` slider changes default delay for newly created robots.
- `Trace new robots` toggles default trace color (`cyan` when enabled).
- `Performance mode` toggles fast rendering mode in the grid.

### Debugger

- `Step mode (pause each action)` makes robot actions wait for permission.
- `Step once` gives one action token.
- `Reset step counter` clears debugger counters.
- Live debug panel shows mode, waiting state, steps, pending tokens, and last action.

### Grid Edit

- Edit modes:
- `View only`
- `Beepers (+ left / - right)`
- `Toggle east wall`
- `Toggle north wall`

- In beeper mode:
- Left click adds one beeper.
- Secondary click removes one beeper.

- `Clear robots` removes all robots.
- `Clear world items` keeps size but clears walls/beepers.

## 6. Built-In Demos

Registered in `demos/demos.go`:

- `Square`: basic loops and turning.
- `Hurdles`: obstacle navigation with helper routine.
- `Harvest`: collect and redistribute beepers.
- `Maze`: deterministic path through fixed maze layout.
- `Goroutine race`: multiple robots moving concurrently.

Use demos to teach control flow, decomposition, and concurrency patterns.

## 7. World File Format (`.wld`)

Parser and writer are implemented in `robot/parser.go`.

### Example

```python
avenues = 10
streets = 10
walls = [
    (6, 1),
    (12, 1),
]
beepers = {
    (3, 2): 2,
    (8, 9): 1,
}
```

### Notes

- `avenues` and `streets` set world size.
- `walls` uses tuple coordinates compatible with classic Python world files.
- `beepers` maps `(avenue, street)` to count.
- Save output from this app preserves the same data model.

Bundled examples are in `worlds/`:

- `empty_10x10.wld`
- `hurdles_track.wld`
- `beeper_garden.wld`
- `maze_runner_1.wld`

## 8. Architecture For Contributors

### Entrypoints

- `main.go`: full interactive app with code execution.
- `cmd/hubo_demo/main.go`: compiled-only demo flow.

### Execution Engine

- `engine/yaegi.go`: initializes Yaegi, loads stdlib, exports robot symbols, runs user code.

### Domain Model

- `robot/world.go`: world dimensions, walls, beepers, snapshots.
- `robot/robot.go`: robot behavior, movement, sensors, trace, step mode.
- `robot/introspection.go`: helper functions for diagnostics and summaries.
- `robot/parser.go`: import/export `.wld` worlds.

### UI Layer

- `ui/window.go`: app layout, controls, world inspector/debug panels.
- `ui/grid.go`: grid renderer, wall/beeper drawing, click-to-edit handling.

### Assets

- `assets/embed.go`: embeds robot PNG sprites.

## 9. Extending The Project

### Add a new demo

1. Add `StartYourDemo()` in `demos/demos.go`.
2. Append `{Label: "Your Demo", Run: StartYourDemo}` in `demos.Actions()`.

### Add a new prebuilt world

1. Save `.wld` file to `worlds/`.
2. Use `Refresh prebuilt list` in the UI.

### Expose more API to interpreted code

1. Add symbol entries in `engine/yaegi.go` (`robotSymbolTable`).
2. Re-run app and validate code snippets in editor.

## 10. Troubleshooting

### GUI does not open on Linux

Install Fyne runtime prerequisites for your distro (OpenGL/windowing libs), then rerun `go run .`.

### Robot does not move

- Check if `Step mode` is enabled and waiting for `Step once`.
- Check for border/wall collisions (`FrontClear()` can help).

### Beeper operations fail

- `PickBeeper()` requires at least one beeper on current cell.
- `DropBeeper()` requires at least one carried beeper.

### Code runs but wrong world appears

- Verify selected world preset/prebuilt world.
- Confirm load path when opening external `.wld` files.

## 11. Suggested Verification Before Commit

```bash
go fmt ./...
go build ./...
go test ./...
```

`go test ./...` is still useful even with minimal test files because it validates package builds.
