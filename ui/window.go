package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/yesetoda/cs1gobot/robot"
)

type DemoAction struct {
	Label string
	Run   func()
}

var GlobalGrid *RobotGrid
var refreshRequested atomic.Bool
var refreshQueued atomic.Bool
var forceFrameLoopStarted atomic.Bool
var frameRefreshInFlight atomic.Bool

func startForcedFrameLoop() {
	if !forceFrameLoopStarted.CompareAndSwap(false, true) {
		return
	}

	go func() {
		ticker := time.NewTicker(time.Second / 60)
		defer ticker.Stop()

		for range ticker.C {
			if GlobalGrid == nil || fyne.CurrentApp() == nil {
				continue
			}
			if !frameRefreshInFlight.CompareAndSwap(false, true) {
				continue
			}

			fyne.Do(func() {
				if GlobalGrid != nil {
					canvas.Refresh(GlobalGrid)
				}
				frameRefreshInFlight.Store(false)
			})
		}
	}()
}

func queueRefresh() {
	if !refreshQueued.CompareAndSwap(false, true) {
		return
	}

	fyne.Do(func() {
		if GlobalGrid != nil && refreshRequested.Swap(false) {
			GlobalGrid.Refresh()
			canvas.Refresh(GlobalGrid)
		} else {
			refreshRequested.Store(false)
		}

		refreshQueued.Store(false)
		if refreshRequested.Load() {
			queueRefresh()
		}
	})
}

// RequestGridRefresh schedules one UI-thread refresh and coalesces bursts
// of world updates (for example during robot animation).
func RequestGridRefresh() {
	if GlobalGrid == nil || fyne.CurrentApp() == nil {
		return
	}
	refreshRequested.Store(true)
	queueRefresh()
}

func directionLabel(dir int) string {
	switch dir {
	case int(robot.North):
		return "N"
	case int(robot.West):
		return "W"
	case int(robot.South):
		return "S"
	default:
		return "E"
	}
}

func buildStepDebuggerText() string {
	state := robot.GetStepDebugState()

	mode := "OFF"
	if state.Enabled {
		mode = "ON"
	}

	waiting := "no"
	if state.Waiting {
		waiting = "yes"
	}

	last := state.LastAction
	if last == "" {
		last = "(none)"
	}

	return fmt.Sprintf(
		"Step mode: %s\nWaiting for step: %s\nSteps executed: %d\nPending step tokens: %d\nLast action: %s",
		mode,
		waiting,
		state.Steps,
		state.Pending,
		last,
	)
}

func buildWorldInspectorText(maxBeeperEntries, maxRobotEntries int) string {
	if robot.CurrentWorld == nil {
		return "World: (none)"
	}

	av, st := robot.CurrentWorld.GetAvenuesAndStreets()
	walls, beepers := robot.CurrentWorld.GetSnapshot()

	totalBeepers := 0
	points := make([]robot.Point, 0, len(beepers))
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

	robot.RegistryMu.Lock()
	robots := make([]*robot.Robot, len(robot.Registry))
	copy(robots, robot.Registry)
	robot.RegistryMu.Unlock()

	var b strings.Builder
	fmt.Fprintf(&b, "World: %dx%d\n", av, st)
	fmt.Fprintf(&b, "Walls: %d\n", len(walls))
	fmt.Fprintf(&b, "Beeper piles: %d\n", len(beepers))
	fmt.Fprintf(&b, "Beepers remaining: %d\n", totalBeepers)

	if len(points) == 0 {
		b.WriteString("Beeper locations: (none)\n")
	} else {
		b.WriteString("Beeper locations (x,y=count):\n")
		limit := maxBeeperEntries
		if limit <= 0 {
			limit = 1
		}
		for i, pt := range points {
			if i >= limit {
				fmt.Fprintf(&b, "... +%d more\n", len(points)-limit)
				break
			}
			fmt.Fprintf(&b, "(%d,%d)=%d\n", pt.X, pt.Y, beepers[pt])
		}
	}

	fmt.Fprintf(&b, "Robots: %d\n", len(robots))
	if len(robots) > 0 {
		limit := maxRobotEntries
		if limit <= 0 {
			limit = 1
		}
		for i, ro := range robots {
			if i >= limit {
				fmt.Fprintf(&b, "... +%d more\n", len(robots)-limit)
				break
			}
			x, y, dir, col := ro.GetState()
			fmt.Fprintf(&b, "R%d %s @(%d,%d) %s\n", i+1, col, x, y, directionLabel(dir))
		}
	}

	return b.String()
}

func discoverPrebuiltWorlds() ([]string, map[string]string) {
	roots := []string{"worlds", filepath.Join("go_robots", "worlds")}
	seen := map[string]bool{}
	labels := make([]string, 0)
	paths := make(map[string]string)

	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.ToLower(filepath.Ext(name)) != ".wld" {
				continue
			}
			full := filepath.Join(root, name)
			if seen[full] {
				continue
			}
			seen[full] = true

			label := strings.TrimSuffix(name, filepath.Ext(name))
			if _, exists := paths[label]; exists {
				label = fmt.Sprintf("%s (%s)", label, root)
			}
			labels = append(labels, label)
			paths[label] = full
		}
	}

	sort.Strings(labels)
	return labels, paths
}

func MakeEditorUI(initialCode string, onRun func(code string)) fyne.CanvasObject {
	editor := widget.NewMultiLineEntry()
	editor.Text = initialCode
	editor.TextStyle.Monospace = true

	runBtn := widget.NewButton("Run Code", func() {
		if onRun != nil {
			onRun(editor.Text)
		}
	})
	if onRun == nil {
		runBtn.Disable()
	}

	editorContainer := container.NewBorder(nil, runBtn, nil, nil, editor)
	return editorContainer
}

func MakeWindowContent(parent fyne.Window, initialCode string, onRun func(code string), demoActions ...DemoAction) fyne.CanvasObject {
	GlobalGrid = NewRobotGrid()
	startForcedFrameLoop()
	worldInfoDirty := &atomic.Bool{}
	worldInfo := widget.NewLabel("")
	worldInfo.TextStyle.Monospace = true
	worldInfo.Wrapping = fyne.TextWrapWord
	debugInfo := widget.NewLabel("")
	debugInfo.TextStyle.Monospace = true
	debugInfo.Wrapping = fyne.TextWrapWord
	debugInfo.SetText(buildStepDebuggerText())

	// Helper to (re)attach world update callback and refresh the grid.
	attachWorld := func() {
		if robot.CurrentWorld == nil {
			return
		}
		robot.UpdateUI = func() {
			RequestGridRefresh()
		}
		robot.CurrentWorld.SetUpdateFunc(func() {
			if robot.UpdateUI != nil {
				robot.UpdateUI()
			}
		})
		worldInfoDirty.Store(true)
		RequestGridRefresh()
	}

	go func() {
		ticker := time.NewTicker(350 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			if !worldInfoDirty.Swap(false) {
				continue
			}
			text := buildWorldInspectorText(14, 8)
			fyne.Do(func() {
				worldInfo.SetText(text)
			})
		}
	}()

	go func() {
		ticker := time.NewTicker(120 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			text := buildStepDebuggerText()
			fyne.Do(func() {
				debugInfo.SetText(text)
			})
		}
	}()

	attachWorld()

	editorUI := MakeEditorUI(initialCode, onRun)

	// --- Interactive controls on the editor side ---

	// World selector: quick presets for common teaching worlds.
	worldSelect := widget.NewSelect([]string{
		"Empty 10x10",
		"Empty 12x8",
		"Empty 20x10",
	}, func(label string) {
		switch label {
		case "Empty 10x10":
			robot.CreateWorld(10, 10)
		case "Empty 12x8":
			robot.CreateWorld(12, 8)
		case "Empty 20x10":
			robot.CreateWorld(20, 10)
		default:
			return
		}
		robot.Reset()
		worldInfoDirty.Store(true)
		attachWorld()
	})
	worldSelect.PlaceHolder = "World..."

	prebuiltLabels, prebuiltPaths := discoverPrebuiltWorlds()
	prebuiltSelect := widget.NewSelect(prebuiltLabels, func(label string) {
		if label == "" {
			return
		}
		path, ok := prebuiltPaths[label]
		if !ok {
			return
		}

		robot.SetStop(true)
		time.Sleep(25 * time.Millisecond)
		robot.SetStop(false)
		robot.Reset()

		if err := robot.LoadWorld(path); err != nil {
			if parent != nil {
				dialog.ShowError(err, parent)
			}
			return
		}
		worldInfoDirty.Store(true)
		attachWorld()
	})
	prebuiltSelect.PlaceHolder = "Prebuilt world..."

	reloadPrebuiltBtn := widget.NewButton("Refresh prebuilt list", func() {
		labels, paths := discoverPrebuiltWorlds()
		prebuiltLabels = labels
		prebuiltPaths = paths
		prebuiltSelect.Options = labels
		prebuiltSelect.ClearSelected()
		prebuiltSelect.Refresh()
	})

	loadFileBtn := widget.NewButton("Load .wld file", func() {
		if parent == nil {
			return
		}

		dlg := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, parent)
				return
			}
			if reader == nil {
				return
			}

			uri := reader.URI()
			_ = reader.Close()
			if uri == nil {
				return
			}

			robot.SetStop(true)
			time.Sleep(25 * time.Millisecond)
			robot.SetStop(false)
			robot.Reset()

			if err := robot.LoadWorld(uri.Path()); err != nil {
				dialog.ShowError(err, parent)
				return
			}
			worldInfoDirty.Store(true)
			attachWorld()
		}, parent)
		dlg.SetFilter(storage.NewExtensionFileFilter([]string{".wld"}))
		if loc, err := storage.ListerForURI(storage.NewFileURI(".")); err == nil {
			dlg.SetLocation(loc)
		}
		dlg.Show()
	})

	saveFileBtn := widget.NewButton("Save world to .wld", func() {
		if parent == nil {
			return
		}

		dlg := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowError(err, parent)
				return
			}
			if writer == nil {
				return
			}

			uri := writer.URI()
			_ = writer.Close()
			if uri == nil {
				return
			}

			path := uri.Path()
			if strings.ToLower(filepath.Ext(path)) != ".wld" {
				path += ".wld"
			}
			if err := robot.SaveWorld(path); err != nil {
				dialog.ShowError(err, parent)
				return
			}
			dialog.ShowInformation("World saved", fmt.Sprintf("Saved to:\n%s", path), parent)
		}, parent)
		dlg.SetFileName("world.wld")
		dlg.SetFilter(storage.NewExtensionFileFilter([]string{".wld"}))
		if loc, err := storage.ListerForURI(storage.NewFileURI(".")); err == nil {
			dlg.SetLocation(loc)
		}
		dlg.Show()
	})

	// Speed slider controls the default delay for new robots.
	speedSlider := widget.NewSlider(0, 1000)
	speedSlider.Step = 50
	speedSlider.Value = float64(robot.DefaultDelayMs)
	speedSlider.OnChanged = func(v float64) {
		robot.SetDefaultDelay(int(v))
	}

	speedLabel := widget.NewLabel("Speed (ms per step)")

	// Trace toggle controls default tracing for new robots.
	traceCheck := widget.NewCheck("Trace new robots", func(on bool) {
		if on {
			robot.SetDefaultTraceColor("cyan")
		} else {
			robot.SetDefaultTraceColor("")
		}
	})
	traceCheck.SetChecked(robot.DefaultTraceColor != "")

	stepModeCheck := widget.NewCheck("Step mode (pause each action)", func(on bool) {
		robot.SetStepMode(on)
	})
	stepModeCheck.SetChecked(robot.GetStepDebugState().Enabled)

	stepOnceBtn := widget.NewButton("Step once", func() {
		if !stepModeCheck.Checked {
			stepModeCheck.SetChecked(true)
		}
		robot.StepOnce()
	})

	stepResetBtn := widget.NewButton("Reset step counter", func() {
		robot.ResetStepperState()
	})

	perfCheck := widget.NewCheck("Performance mode", func(on bool) {
		if GlobalGrid == nil {
			return
		}
		GlobalGrid.SetFastMode(on)
		RequestGridRefresh()
	})
	perfCheck.SetChecked(true)

	editModeSelect := widget.NewSelect(GridEditModeOptions(), func(label string) {
		if GlobalGrid == nil {
			return
		}
		GlobalGrid.SetEditMode(GridEditModeFromLabel(label))
	})
	editModeSelect.SetSelected(GridEditModeOptions()[1])

	clearRobotsBtn := widget.NewButton("Clear robots", func() {
		robot.SetStop(true)
		time.Sleep(25 * time.Millisecond)
		robot.SetStop(false)
		robot.Reset()
		RequestGridRefresh()
	})

	clearWorldBtn := widget.NewButton("Clear world items", func() {
		if robot.CurrentWorld == nil {
			return
		}
		av, st := robot.CurrentWorld.GetAvenuesAndStreets()
		robot.CreateWorld(av, st)
		robot.Reset()
		worldInfoDirty.Store(true)
		attachWorld()
	})

	worldInfoScroll := container.NewVScroll(worldInfo)
	worldInfoScroll.SetMinSize(fyne.NewSize(260, 170))

	worldSetupBox := container.NewVBox(
		worldSelect,
		prebuiltSelect,
		reloadPrebuiltBtn,
		loadFileBtn,
		saveFileBtn,
	)

	simulationBox := container.NewVBox(
		speedLabel,
		speedSlider,
		perfCheck,
		traceCheck,
	)

	debugInfoScroll := container.NewVScroll(debugInfo)
	debugInfoScroll.SetMinSize(fyne.NewSize(260, 130))

	debuggerBox := container.NewVBox(
		stepModeCheck,
		container.NewGridWithColumns(2, stepOnceBtn, stepResetBtn),
		debugInfoScroll,
	)

	gridEditBox := container.NewVBox(
		editModeSelect,
		clearRobotsBtn,
		clearWorldBtn,
	)

	accordionItems := []*widget.AccordionItem{
		widget.NewAccordionItem("World Setup", worldSetupBox),
		widget.NewAccordionItem("Simulation", simulationBox),
		widget.NewAccordionItem("Debugger", debuggerBox),
		widget.NewAccordionItem("Grid Edit", gridEditBox),
		widget.NewAccordionItem("World Details", worldInfoScroll),
	}

	if len(demoActions) > 0 {
		demoButtons := make([]fyne.CanvasObject, 0, len(demoActions))
		for _, action := range demoActions {
			a := action
			if a.Label == "" || a.Run == nil {
				continue
			}
			demoButtons = append(demoButtons, widget.NewButton(a.Label, func() {
				robot.SetStop(true)
				time.Sleep(25 * time.Millisecond)
				robot.SetStop(false)
				robot.Reset()
				a.Run()
				attachWorld()
			}))
		}
		if len(demoButtons) > 0 {
			accordionItems = append(accordionItems, widget.NewAccordionItem("Demos", container.NewVBox(demoButtons...)))
		}
	}

	controlsAccordion := widget.NewAccordion(accordionItems...)
	controlsAccordion.MultiOpen = true
	if len(controlsAccordion.Items) > 0 {
		controlsAccordion.Open(0)
	}

	controlsTip := widget.NewLabel("Tip: click section titles to expand/collapse. Drag the divider to resize controls/editor.")
	controlsTip.Wrapping = fyne.TextWrapWord
	controlsPanel := container.NewVBox(controlsTip, controlsAccordion)

	controlsScroll := container.NewVScroll(controlsPanel)
	controlsScroll.SetMinSize(fyne.NewSize(260, 160))

	right := container.NewVSplit(controlsScroll, editorUI)
	right.Offset = 0.38

	split := container.NewHSplit(GlobalGrid, right)
	split.Offset = 0.6 // 60% for grid, 40% for editor
	worldInfoDirty.Store(true)

	return split
}
