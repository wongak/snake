package main // import "github.com/wongak/snake"

import (
	"errors"
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten"
)

const (
	title = "snake"
)

var (
	width  = 400
	height = 400

	cellsX = 100
	cellsY = 100

	initialLength = 3

	speed float64 = 30.0
)

var (
	bgColor = color.RGBA{0x18, 0x29, 0x18, 0xff}
	snColor = color.RGBA{0x20, 0xff, 0x20, 0xff}
	errEnd  = errors.New("end")
)

type world struct {
	screenW, screenH int
	cellsX, cellsY   int
	cellW, cellH     int
	tile             *ebiten.Image
}

func newWorld(w, h, x, y int) *world {
	world := &world{
		screenW: w,
		screenH: h,
		cellsX:  x,
		cellsY:  y,
		cellW:   w / x,
		cellH:   h / y,
	}
	world.tile, _ = ebiten.NewImage(world.cellW, world.cellH, ebiten.FilterNearest)
	world.tile.Fill(snColor)
	return world
}

type node struct {
	child, parent *node
	x, y          int
}

func (n *node) draw(w *world, canvas *ebiten.Image) {
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Translate(float64(w.cellW*n.x), float64(w.cellH*n.y))
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

var (
	w      *world
	h      *head
	frame  int64
	points int
)

func main() {
	w = newWorld(width, height, cellsX, cellsY)
	h = initSnake(w, initialLength)
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
	if ebiten.IsKeyPressed(ebiten.KeyUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
		h.direction = 3
	}
	if ebiten.IsKeyPressed(ebiten.KeyDown) || ebiten.IsKeyPressed(ebiten.KeyS) {
		h.direction = 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		h.direction = 2
	}
	if ebiten.IsKeyPressed(ebiten.KeyRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		h.direction = 0
	}

	if frame%int64(speed) == 0 {
		h.move(w, h.direction)
	}

	screen.Fill(bgColor)
	h.draw(w, screen)

	return nil
}
