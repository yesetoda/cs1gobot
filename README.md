# Go Robots

Go Robots is a desktop learning environment for grid-based robot programming in Go.
It combines a visual world, an in-app code editor, and a Yaegi-powered interpreter so learners can write code and immediately watch robots move.

## What You Get

- Interactive 2D world with walls, beepers, and animated robots.
- Built-in editor for writing and running Go code.
- Ready-made demos (Square, Hurdles, Harvest, Maze, Goroutine race).
- Step debugger for action-by-action execution.
- World editor with load/save support for `.wld` files.
- Compiled demo mode (`cmd/hubo_demo`) for presentation or packaged usage.

## Requirements

- Go `1.22` or newer.
- Desktop environment capable of running Fyne GUI apps.
- Platform prerequisites required by Fyne (OpenGL/windowing dependencies on Linux).

Reference: the project uses `fyne.io/fyne/v2` for the UI and `github.com/traefik/yaegi` for runtime code execution.

## Linux Setup

Because this project uses Fyne for a desktop GUI, some Linux systems need extra native development packages before `go run .` or `go build ./...` will work.

### Fedora

If you are on Fedora, install the required packages with:

```bash
sudo dnf install golang gcc libX11-devel libXcursor-devel libXrandr-devel libXinerama-devel libXi-devel libXxf86vm-devel mesa-libGL-devel
```

If Go is already installed on your machine, you can omit `golang` and keep the remaining packages.

### Other Linux distributions

Package names vary by distro, but you will usually need the equivalents of:

- Go
- GCC
- X11 development headers
- `libXcursor`, `libXrandr`, `libXinerama`, `libXi`, and `libXxf86vm` development packages
- Mesa/OpenGL development packages

## Quick Start

From the repository root:

```bash
go mod download
go run .
```

This launches the full learning UI (grid + controls + code editor).

## Run Modes

### 1) Full Interactive Mode (Default)

```bash
go run .
```

- Includes code editor and `Run Code` button.
- Runs your code through Yaegi.
- Exposes world setup, debugger, and grid editing controls.

### 2) Compiled Demo Mode

```bash
go run ./cmd/hubo_demo
```

- Same UI surface, but editor run action is disabled.
- Starts with a built-in example and auto-runs a demo.
- Useful for classroom display, kiosk mode, or packaging.

## Build

```bash
go build ./...
```

Optional binaries:

```bash
go build -o go-robots .
go build -o go-robots-demo ./cmd/hubo_demo
```

## First Program

Paste this in the editor and click `Run Code`:

```go
package main

import "github.com/yesetoda/cs1gobot/robot"

func main() {
    hubo := robot.New("blue")
    hubo.SetDelay(120)
    hubo.SetTrace("cyan")

    for i := 0; i < 4; i++ {
        for s := 0; s < 4; s++ {
            hubo.Move()
        }
        hubo.TurnLeft()
    }
}
```

## UI Controls At A Glance

- `World Setup`
- Choose empty world presets.
- Load prebuilt worlds from `worlds/`.
- Load or save `.wld` files.

- `Simulation`
- Set default robot delay with the speed slider.
- Toggle trace for newly created robots.
- Toggle performance mode rendering.

- `Debugger`
- Enable step mode (pause every robot action).
- Advance one action with `Step once`.
- Reset step counters.

- `Grid Edit`
- Add/remove beepers.
- Toggle east/north walls.
- Clear robots or clear world items.

- `Demos`
- Run built-in scenarios to explore patterns and concurrency.

## Project Layout

```text
.
|-- main.go                 # Full app entrypoint (interactive + interpreter)
|-- cmd/hubo_demo/main.go   # Compiled demo entrypoint
|-- engine/yaegi.go         # Yaegi setup and robot symbol export
|-- robot/
|   |-- robot.go            # Robot behavior and step control
|   |-- world.go            # World model, walls, beepers
|   |-- parser.go           # .wld load/save
|   `-- introspection.go    # World summary/introspection helpers
|-- ui/
|   |-- window.go           # Main window layout and controls
|   `-- grid.go             # Grid rendering and click-to-edit behavior
|-- demos/
|   |-- demos.go            # Built-in demo actions
|   `-- demo2.go            # DFS exploration example
|-- worlds/                 # Bundled world files
`-- assets/                 # Embedded robot sprite PNG files
```

## Detailed Guide

For full API reference, world format details, debugger behavior, architecture notes, and extension instructions, see `GUIDE.md`.

## Troubleshooting

- If a robot appears stuck, check whether `Step mode` is enabled.
- If execution fails with a robot error, inspect walls/borders and beeper conditions.
- If the GUI does not start on Linux, verify the Fyne platform dependencies above are installed first. On Fedora, use the `dnf` command in `Linux Setup`.

## Notes For Git Publishing

Before your first push:

```bash
go mod tidy
go build ./...
go test ./...
```

If tests are not added yet, `go test ./...` still verifies package compilation.
