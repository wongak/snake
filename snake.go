package main

import (
	"errors"
	"image/color"
	"log"
	"math"
	"math/rand"
	"strconv"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/text"
	"golang.org/x/image/font/basicfont"
)

const (
	title = "snake"
)

var (
	width  = 400
	height = 400

	cellsX = 50
	cellsY = 50

	initialLength = 3

	speed float64 = 12.0
)

var (
	bgColor     = color.RGBA{0x18, 0x29, 0x18, 0xff}
	borderColor = color.RGBA{0x10, 0xa0, 0x10, 0xff}
	snColor     = color.RGBA{0x20, 0xff, 0x20, 0xff}
	foodColor   = color.RGBA{0xa0, 0xa0, 0x10, 0xff}
	errEnd      = errors.New("end")
	errLose     = errors.New("lose")
	opts        = &ebiten.DrawImageOptions{}
)

type world struct {
	screenW, screenH int
	cellsX, cellsY   int
	cellW, cellH     int
	tile             *ebiten.Image
	foodTile         *ebiten.Image

	borders *ebiten.Image
}

func newWorld(w, h, x, y int) *world {
	world := &world{
		screenW: w,
		screenH: h,
		cellsX:  x,
		cellsY:  y,
		// cell size in pixels is at least width / (cells + 1),
		// otherwise the last cell is outside of the screen
		// + 2 for drawing a border on row/column 0
		cellW: w / (x + 12),
		cellH: h / (y + 12),
	}
	world.tile, _ = ebiten.NewImage(world.cellW, world.cellH, ebiten.FilterNearest)
	world.tile.Fill(snColor)
	world.foodTile, _ = ebiten.NewImage(world.cellW, world.cellH, ebiten.FilterNearest)
	world.foodTile.Fill(foodColor)

	world.initBorders()
	return world
}

func (w *world) initBorders() {
	w.borders, _ = ebiten.NewImage(w.screenW, w.screenH, ebiten.FilterNearest)
	hor, _ := ebiten.NewImage(w.cellW*(w.cellsX+2), w.cellH, ebiten.FilterNearest)
	hor.Fill(borderColor)
	w.borders.DrawImage(hor, opts)
	opts.GeoM.Reset()
	opts.GeoM.Translate(0, float64(w.cellH*(w.cellsY+2)))
	w.borders.DrawImage(hor, opts)
	vert, _ := ebiten.NewImage(w.cellW, w.cellH*(w.cellsY+3), ebiten.FilterNearest)
	vert.Fill(borderColor)
	opts.GeoM.Reset()
	w.borders.DrawImage(vert, opts)
	opts.GeoM.Translate(float64(w.cellW*(w.cellsX+2)), 0)
	w.borders.DrawImage(vert, opts)
}

func (w *world) draw(canvas *ebiten.Image) {
	canvas.DrawImage(w.borders, &ebiten.DrawImageOptions{})
}

type node struct {
	child, parent *node
	x, y          int
}

func (n *node) draw(w *world, canvas *ebiten.Image) {
	opts.GeoM.Reset()
	opts.GeoM.Translate(float64(w.cellW*(n.x+1)), float64(w.cellH*(n.y+1)))
	canvas.DrawImage(w.tile, opts)
	if n.child != nil {
		n.child.draw(w, canvas)
	}
}

func (n *node) step(w *world) {
	if n.child != nil {
		n.child.step(w)
	}
	n.x = n.parent.x
	n.y = n.parent.y
	if n.child == nil && grow > 0 {
		curr := n
		for ; grow > 0; grow-- {
			curr.child = &node{parent: curr, x: curr.x, y: curr.y}
			curr = curr.child
		}
	}
}

func (n *node) collided(x, y int) bool {
	if x == n.x && y == n.y {
		return true
	}
	if n.child != nil {
		return n.child.collided(x, y)
	}
	return false
}

type head struct {
	*node
	direction int
}

func (h *head) move(w *world, direction int) {
	if h.child != nil {
		h.child.step(w)
	}
	switch direction % 4 {
	case 0:
		h.x += 1
	case 1:
		h.y += 1
	case 2:
		h.x -= 1
	case 3:
		h.y -= 1
	}
	if h.x < 0 {
		h.x = w.cellsX
	}
	if h.x > w.cellsX {
		h.x = 0
	}
	if h.y < 0 {
		h.y = w.cellsY
	}
	if h.y > w.cellsY {
		h.y = 0
	}
}

func (h *head) alive() bool {
	if h.child == nil {
		return true
	}
	return !h.child.collided(h.x, h.y)
}

func initSnake(w *world, initialLength int) *head {
	head := &head{
		node: &node{
			x: w.cellsX / 2,
			y: w.cellsY / 2,
		},
	}
	currNode := head.node
	for i := 1; i < initialLength; i++ {
		node := &node{
			x: currNode.x - 1,
			y: currNode.y,
		}
		currNode.child = node
		node.parent = currNode
		currNode = node
	}
	return head
}

type food struct {
	x, y int
}

func (f *food) respawn(w *world) {
	var x, y int
	for {
		x = rand.Intn(w.cellsX)
		y = rand.Intn(w.cellsY)
		if h.collided(x, y) {
			continue
		}
		break
	}
	f.x = x
	f.y = y
}

func (f *food) draw(w *world, canvas *ebiten.Image) {
	opts.GeoM.Reset()
	opts.GeoM.Translate(float64(w.cellW*(f.x+1)), float64(w.cellH*(f.y+1)))
	canvas.DrawImage(w.foodTile, opts)
}

func drawPoints(w *world, canvas *ebiten.Image) {
	text.Draw(canvas, strconv.FormatInt(points, 10), basicfont.Face7x13, w.cellW, w.cellH*(w.cellsY+6), snColor)
}

var (
	w      *world
	h      *head
	f      *food
	grow   int = 1
	moving bool
	frame  int64
	points int64
)

func main() {
	w = newWorld(width, height, cellsX, cellsY)
	h = initSnake(w, initialLength)
	f = &food{}
	f.respawn(w)
	if err := ebiten.Run(update, width, height, 2, title); err != nil {
		if err == errEnd {
			return
		}
		log.Fatal(err)
	}
}

func update(screen *ebiten.Image) error {
	frame++
	if ebiten.IsRunningSlowly() {
		// frame skip
		return nil
	}
	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return errEnd
	}
	if (ebiten.IsKeyPressed(ebiten.KeyUp) || ebiten.IsKeyPressed(ebiten.KeyW)) &&
		h.direction%4 != 1 {
		h.direction = 3
	}
	if (ebiten.IsKeyPressed(ebiten.KeyDown) || ebiten.IsKeyPressed(ebiten.KeyS)) &&
		h.direction%4 != 3 {
		h.direction = 1
	}
	if (ebiten.IsKeyPressed(ebiten.KeyLeft) || ebiten.IsKeyPressed(ebiten.KeyA)) &&
		h.direction%4 != 0 {
		h.direction = 2
	}
	if (ebiten.IsKeyPressed(ebiten.KeyRight) || ebiten.IsKeyPressed(ebiten.KeyD)) &&
		h.direction%4 != 2 {
		h.direction = 0
	}
	currSpeed := speed - (float64(points) / 10000.0)

	if frame%int64(currSpeed) == 0 {
		h.move(w, h.direction)
		if !h.alive() {
			return errLose
		}
		points += 10
	}
	// eat
	if h.node.x == f.x && h.node.y == f.y {
		points += 1000
		grow = int(math.Log10(float64(points)))
		f.respawn(w)
	}

	screen.Fill(bgColor)
	w.draw(screen)
	h.draw(w, screen)
	if f != nil {
		f.draw(w, screen)
	}
	drawPoints(w, screen)

	return nil
}
