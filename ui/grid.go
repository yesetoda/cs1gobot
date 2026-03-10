package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	robotassets "github.com/yesetoda/cs1gobot/assets"
	"github.com/yesetoda/cs1gobot/robot"
)

type GridEditMode int

const (
	GridEditNone GridEditMode = iota
	GridEditBeeper
	GridEditWallEast
	GridEditWallNorth
)

var gridEditModeLabels = []string{
	"View only",
	"Beepers (+ left / - right)",
	"Toggle east wall",
	"Toggle north wall",
}

const maxRenderedTraceSegments = 80

var robotAssetResourceCache sync.Map
var generatedRobotResourceCache sync.Map
var baseDirectionalImageCache sync.Map

func GridEditModeOptions() []string {
	res := make([]string, len(gridEditModeLabels))
	copy(res, gridEditModeLabels)
	return res
}

func GridEditModeFromLabel(label string) GridEditMode {
	switch label {
	case gridEditModeLabels[1]:
		return GridEditBeeper
	case gridEditModeLabels[2]:
		return GridEditWallEast
	case gridEditModeLabels[3]:
		return GridEditWallNorth
	default:
		return GridEditNone
	}
}

type RobotGrid struct {
	widget.BaseWidget

	mu       sync.RWMutex
	editMode GridEditMode
	fastMode bool
}

func NewRobotGrid() *RobotGrid {
	g := &RobotGrid{editMode: GridEditBeeper, fastMode: true}
	g.ExtendBaseWidget(g)
	return g
}

func (g *RobotGrid) SetEditMode(mode GridEditMode) {
	g.mu.Lock()
	g.editMode = mode
	g.mu.Unlock()
}

func (g *RobotGrid) SetFastMode(on bool) {
	g.mu.Lock()
	g.fastMode = on
	g.mu.Unlock()
}

func (g *RobotGrid) pointFromPosition(pos fyne.Position) (robot.Point, bool) {
	world := robot.CurrentWorld
	if world == nil {
		return robot.Point{}, false
	}

	av, st := world.GetAvenuesAndStreets()
	if av <= 0 || st <= 0 {
		return robot.Point{}, false
	}

	size := g.Size()
	offsetX, offsetY, spacing := computeGridLayout(size, av, st)
	if spacing <= 0 {
		return robot.Point{}, false
	}

	xf := float64((pos.X - offsetX) / spacing)
	yf := float64((size.Height - pos.Y - offsetY) / spacing)
	x := int(math.Floor(xf)) + 1
	y := int(math.Floor(yf)) + 1

	if x < 1 || x > av || y < 1 || y > st {
		return robot.Point{}, false
	}

	return robot.Point{X: x, Y: y}, true
}

func (g *RobotGrid) editAt(pos fyne.Position, secondary bool) {
	world := robot.CurrentWorld
	if world == nil {
		return
	}

	pt, ok := g.pointFromPosition(pos)
	if !ok {
		return
	}

	g.mu.RLock()
	mode := g.editMode
	g.mu.RUnlock()

	switch mode {
	case GridEditBeeper:
		if secondary {
			world.RemoveBeeper(pt.X, pt.Y)
		} else {
			world.AddBeeper(pt.X, pt.Y)
		}
	case GridEditWallEast:
		av, _ := world.GetAvenuesAndStreets()
		if pt.X >= av {
			return
		}
		world.ToggleWall(pt, robot.Point{X: pt.X + 1, Y: pt.Y})
	case GridEditWallNorth:
		_, st := world.GetAvenuesAndStreets()
		if pt.Y >= st {
			return
		}
		world.ToggleWall(pt, robot.Point{X: pt.X, Y: pt.Y + 1})
	}

	// Tap handlers run on the UI thread, so refresh immediately for snappy edits.
	g.Refresh()
	RequestGridRefresh()
}

func (g *RobotGrid) Tapped(ev *fyne.PointEvent) {
	g.editAt(ev.Position, false)
}

func (g *RobotGrid) TappedSecondary(ev *fyne.PointEvent) {
	g.editAt(ev.Position, true)
}

func (g *RobotGrid) CreateRenderer() fyne.WidgetRenderer {
	return &gridRenderer{grid: g}
}

func loadRobotAssetResource(assetName string) fyne.Resource {
	if cached, ok := robotAssetResourceCache.Load(assetName); ok {
		if res, okRes := cached.(fyne.Resource); okRes {
			return res
		}
	}

	if data, err := robotassets.Files.ReadFile(assetName); err == nil {
		res := fyne.NewStaticResource(assetName, data)
		robotAssetResourceCache.Store(assetName, res)
		return res
	}

	paths := []string{
		filepath.Join("assets", assetName),
		filepath.Join("go_robots", "assets", assetName),
		assetName,
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			res := fyne.NewStaticResource(assetName, data)
			robotAssetResourceCache.Store(assetName, res)
			return res
		}
	}

	return nil
}

func resolveColorAsset(colorName string) (string, bool) {
	name := strings.ToLower(strings.TrimSpace(colorName))
	if name == "" {
		name = "white"
	}

	exact := name + ".png"
	if loadRobotAssetResource(exact) != nil {
		return exact, false
	}

	switch name {
	case "light_blue":
		if loadRobotAssetResource("blue.png") != nil {
			return "blue.png", true
		}
	case "purple":
		if loadRobotAssetResource("red.png") != nil {
			return "red.png", true
		}
	case "gray", "grey":
		if loadRobotAssetResource("black.png") != nil {
			return "black.png", true
		}
	}

	fallbacks := []string{"white.png", "blue.png", "green.png", "yellow.png", "red.png", "black.png"}
	for _, f := range fallbacks {
		if loadRobotAssetResource(f) != nil {
			return f, true
		}
	}

	return "", true
}

func loadDirectionalBaseImage(assetName string) (*image.NRGBA, error) {

	if cached, ok := baseDirectionalImageCache.Load(assetName); ok {
		if img, okImg := cached.(*image.NRGBA); okImg && img != nil {
			copyImg := image.NewNRGBA(img.Bounds())
			copy(copyImg.Pix, img.Pix)
			return copyImg, nil
		}
	}

	res := loadRobotAssetResource(assetName)
	if res == nil {
		return nil, fmt.Errorf("%s not found", assetName)
	}

	img, _, err := image.Decode(bytes.NewReader(res.Content()))
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(out, out.Bounds(), img, bounds.Min, draw.Src)
	baseDirectionalImageCache.Store(assetName, out)

	copyImg := image.NewNRGBA(out.Bounds())
	copy(copyImg.Pix, out.Pix)
	return copyImg, nil
}

func rotate90CW(src *image.NRGBA) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))

	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			s := src.PixOffset(x, y)
			dx := b.Dy() - 1 - y
			dy := x
			d := dst.PixOffset(dx, dy)
			copy(dst.Pix[d:d+4], src.Pix[s:s+4])
		}
	}

	return dst
}

func clockwiseTurnsFromEast(dir int) int {
	switch dir {
	case int(robot.South):
		return 1
	case int(robot.West):
		return 2
	case int(robot.North):
		return 3
	default:
		return 0
	}
}

func quarterTurnsFromBase(baseDir, targetDir int) int {
	baseTurns := clockwiseTurnsFromEast(baseDir)
	targetTurns := clockwiseTurnsFromEast(targetDir)
	return (targetTurns - baseTurns + 4) % 4
}

func robotTintColor(name string) color.NRGBA {
	switch name {
	case "blue":
		return color.NRGBA{R: 66, G: 135, B: 245, A: 255}
	case "light_blue":
		return color.NRGBA{R: 103, G: 179, B: 255, A: 255}
	case "green":
		return color.NRGBA{R: 46, G: 160, B: 67, A: 255}
	case "purple":
		return color.NRGBA{R: 122, G: 87, B: 209, A: 255}
	case "yellow":
		return color.NRGBA{R: 235, G: 191, B: 44, A: 255}
	default:
		return color.NRGBA{R: 138, G: 138, B: 138, A: 255}
	}
}

func tintRobotImage(src *image.NRGBA, tint color.NRGBA) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))

	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			i := src.PixOffset(x, y)
			r := src.Pix[i]
			g := src.Pix[i+1]
			bl := src.Pix[i+2]
			a := src.Pix[i+3]

			if a == 0 {
				continue
			}

			// Keep shading by using luminance from the source image.
			lum := uint8((uint16(r)*54 + uint16(g)*183 + uint16(bl)*19) / 256)
			tr := uint8((uint16(lum) * uint16(tint.R)) / 255)
			tg := uint8((uint16(lum) * uint16(tint.G)) / 255)
			tb := uint8((uint16(lum) * uint16(tint.B)) / 255)

			// Blend a bit of original detail so facial features remain crisp.
			dst.Pix[i] = uint8((uint16(tr)*224 + uint16(r)*31) / 255)
			dst.Pix[i+1] = uint8((uint16(tg)*224 + uint16(g)*31) / 255)
			dst.Pix[i+2] = uint8((uint16(tb)*224 + uint16(bl)*31) / 255)
			dst.Pix[i+3] = a
		}
	}

	return dst
}

func generateRobotResource(colorName string, dir int) fyne.Resource {
	baseName, applyTint := resolveColorAsset(colorName)
	if baseName == "" {
		return nil
	}

	key := fmt.Sprintf("robot_%s_%s_%d_t%t", colorName, baseName, dir, applyTint)
	if cached, ok := generatedRobotResourceCache.Load(key); ok {
		if res, okRes := cached.(fyne.Resource); okRes {
			return res
		}
	}

	base, err := loadDirectionalBaseImage(baseName)
	if err != nil {
		return nil
	}

	img := base
	if applyTint {
		img = tintRobotImage(base, robotTintColor(colorName))
	}

	// Color assets are north-facing source sprites.
	turns := quarterTurnsFromBase(int(robot.North), dir)
	for i := 0; i < turns; i++ {
		img = rotate90CW(img)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}

	res := fyne.NewStaticResource(key+".png", buf.Bytes())
	generatedRobotResourceCache.Store(key, res)
	return res
}

type gridRenderer struct {
	grid    *RobotGrid
	objects []fyne.CanvasObject

	baseKey     baseCacheKey
	baseObjects []fyne.CanvasObject
}

type baseCacheKey struct {
	av int
	st int
	w  int
	h  int
}

func fastRobotColor(name string) color.Color {
	switch name {
	case "blue", "light_blue":
		return color.RGBA{R: 66, G: 135, B: 245, A: 255}
	case "green":
		return color.RGBA{R: 46, G: 160, B: 67, A: 255}
	case "purple":
		return color.RGBA{R: 122, G: 87, B: 209, A: 255}
	case "yellow":
		return color.RGBA{R: 235, G: 191, B: 44, A: 255}
	default:
		return color.RGBA{R: 120, G: 120, B: 120, A: 255}
	}
}

func (r *gridRenderer) Layout(size fyne.Size) {
	// Layout is handled in Refresh
}

func (r *gridRenderer) MinSize() fyne.Size {
	world := robot.CurrentWorld
	if world == nil {
		return fyne.NewSize(400, 400)
	}
	av, st := world.GetAvenuesAndStreets()
	return fyne.NewSize(float32((av+2)*40), float32((st+2)*40))
}

func (r *gridRenderer) Refresh() {
	world := robot.CurrentWorld
	if world == nil {
		r.objects = nil
		return
	}

	av, st := world.GetAvenuesAndStreets()
	size := r.grid.Size()

	// Constants for styling
	const labelOffset = float32(25)
	offsetX, offsetY, spacing := computeGridLayout(size, av, st)

	toScreen := func(x, y int) fyne.Position {
		return fyne.NewPos(
			offsetX+(float32(x)-0.5)*spacing,
			size.Height-offsetY-(float32(y)-0.5)*spacing,
		)
	}

	r.grid.mu.RLock()
	fastMode := r.grid.fastMode
	r.grid.mu.RUnlock()

	key := baseCacheKey{av: av, st: st, w: int(size.Width), h: int(size.Height)}
	if r.baseKey != key {
		r.baseKey = key
		r.baseObjects = r.baseObjects[:0]

		// 1. Draw cell boundary lines (walls lie on these edges).
		for a := 0; a <= av; a++ {
			x := offsetX + float32(a)*spacing
			pBot := fyne.NewPos(x, size.Height-offsetY)
			pTop := fyne.NewPos(x, size.Height-offsetY-float32(st)*spacing)

			line := canvas.NewLine(theme.DisabledColor())
			line.Position1 = pBot
			line.Position2 = pTop
			r.baseObjects = append(r.baseObjects, line)
		}

		for s := 0; s <= st; s++ {
			y := size.Height - offsetY - float32(s)*spacing
			pLeft := fyne.NewPos(offsetX, y)
			pRight := fyne.NewPos(offsetX+float32(av)*spacing, y)

			line := canvas.NewLine(theme.DisabledColor())
			line.Position1 = pLeft
			line.Position2 = pRight
			r.baseObjects = append(r.baseObjects, line)
		}

		// Avenue/street labels at cell centers.
		for a := 1; a <= av; a++ {
			center := toScreen(a, 1)
			txt := canvas.NewText(fmt.Sprintf("%d", a), theme.ForegroundColor())
			txt.TextSize = 12
			txt.Alignment = fyne.TextAlignCenter
			txt.Move(fyne.NewPos(center.X-10, size.Height-offsetY+labelOffset))
			r.baseObjects = append(r.baseObjects, txt)
		}

		for s := 1; s <= st; s++ {
			center := toScreen(1, s)
			txt := canvas.NewText(fmt.Sprintf("%d", s), theme.ForegroundColor())
			txt.TextSize = 12
			txt.Alignment = fyne.TextAlignTrailing
			txt.Move(fyne.NewPos(offsetX-labelOffset-10, center.Y-10))
			r.baseObjects = append(r.baseObjects, txt)
		}
	}

	objs := make([]fyne.CanvasObject, 0, len(r.baseObjects)+64)
	objs = append(objs, r.baseObjects...)

	// 2. Draw Walls
	walls, beepers := world.GetSnapshot()
	for w := range walls {
		line := canvas.NewLine(color.RGBA{R: 220, G: 20, B: 60, A: 255}) // Crimson
		line.StrokeWidth = 6

		if w.P1.X != w.P2.X {
			x := w.P1.X
			if w.P2.X < x {
				x = w.P2.X
			}
			y := w.P1.Y
			center := toScreen(x, y)
			xEdge := offsetX + float32(x)*spacing
			line.Position1 = fyne.NewPos(xEdge, center.Y-spacing/2)
			line.Position2 = fyne.NewPos(xEdge, center.Y+spacing/2)
		} else {
			y := w.P1.Y
			if w.P2.Y < y {
				y = w.P2.Y
			}
			x := w.P1.X
			center := toScreen(x, y)
			yEdge := size.Height - offsetY - float32(y)*spacing
			line.Position1 = fyne.NewPos(center.X-spacing/2, yEdge)
			line.Position2 = fyne.NewPos(center.X+spacing/2, yEdge)
		}
		objs = append(objs, line)
	}

	// 3. Draw Beepers
	for pt, count := range beepers {
		center := toScreen(pt.X, pt.Y)
		radius := spacing * 0.25
		if radius > 15 {
			radius = 15
		}

		circ := canvas.NewCircle(color.RGBA{R: 255, G: 215, B: 0, A: 255}) // Gold
		circ.StrokeColor = color.RGBA{R: 184, G: 134, B: 11, A: 255}
		circ.StrokeWidth = 2
		circ.Resize(fyne.NewSize(radius*2, radius*2))
		circ.Move(fyne.NewPos(center.X-radius, center.Y-radius))
		objs = append(objs, circ)

		if count > 1 {
			txt := canvas.NewText(fmt.Sprintf("%d", count), color.Black)
			txt.TextSize = 10
			txt.TextStyle.Bold = true
			txt.Move(fyne.NewPos(center.X-5, center.Y-8))
			objs = append(objs, txt)
		}
	}

	// 4. Draw Robots and optional traces
	robot.RegistryMu.Lock()
	robots := make([]*robot.Robot, len(robot.Registry))
	copy(robots, robot.Registry)
	robot.RegistryMu.Unlock()

	if !fastMode {
		for _, ro := range robots {
			path := ro.GetTrace()
			start := 1
			if len(path) > maxRenderedTraceSegments+1 {
				start = len(path) - maxRenderedTraceSegments
			}
			for i := start; i < len(path); i++ {
				p1 := toScreen(path[i-1].X, path[i-1].Y)
				p2 := toScreen(path[i].X, path[i].Y)

				line := canvas.NewLine(color.RGBA{R: 0, G: 191, B: 255, A: 200}) // Deep Sky Blue
				line.StrokeWidth = 3
				line.Position1 = p1
				line.Position2 = p2
				objs = append(objs, line)
			}
		}
	}

	// 5. Draw Robots
	for _, ro := range robots {
		x, y, dir, col := ro.GetState()
		center := toScreen(x, y)

		imgSize := spacing * 0.7
		if imgSize > 48 {
			imgSize = 48
		}
		if imgSize < 24 {
			imgSize = 24
		}

		res := generateRobotResource(col, dir)
		if res == nil {
			assetName, _ := resolveColorAsset(col)
			if assetName != "" {
				res = loadRobotAssetResource(assetName)
			}
		}
		if res == nil {
			radius := imgSize * 0.45
			body := canvas.NewCircle(fastRobotColor(col))
			body.StrokeColor = color.Black
			body.StrokeWidth = 1.5
			body.Resize(fyne.NewSize(radius*2, radius*2))
			body.Move(fyne.NewPos(center.X-radius, center.Y-radius))
			objs = append(objs, body)
			continue
		}

		img := canvas.NewImageFromResource(res)
		img.FillMode = canvas.ImageFillContain
		img.Resize(fyne.NewSize(imgSize, imgSize))
		img.Move(fyne.NewPos(center.X-imgSize/2, center.Y-imgSize/2))
		objs = append(objs, img)
	}

	r.objects = objs
}

func computeGridLayout(size fyne.Size, av, st int) (float32, float32, float32) {
	const margin = float32(50)

	availableWidth := size.Width - 2*margin
	availableHeight := size.Height - 2*margin

	denomX := float32(av)
	if av <= 0 {
		denomX = 1
	}
	denomY := float32(st)
	if st <= 0 {
		denomY = 1
	}

	spacingX := availableWidth / denomX
	spacingY := availableHeight / denomY
	spacing := spacingX
	if spacingY < spacingX {
		spacing = spacingY
	}
	if spacing < 20 {
		spacing = 20
	}

	offsetX := (size.Width - spacing*float32(av)) / 2
	offsetY := (size.Height - spacing*float32(st)) / 2

	return offsetX, offsetY, spacing
}

func (r *gridRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *gridRenderer) Destroy() {
}
