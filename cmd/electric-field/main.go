package main

import (
	"fmt"
	"image/color"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
)

const (
	screenWidth  = 900
	screenHeight = 600

	kConst = 2000.0 // кулоновская константа

	minR2 = 16.0 // r^2

	fieldLineStep   = 3.0  // шаг интегрирования линий поля
	fieldLineMaxLen = 1500 // максимальное кол-во шагов линии
	seedRadius      = 8.0  // стартовая дистанция точки линии от заряда
	seedsPerCharge  = 20   // сколько линий на заряд
	arrowGridStep   = 40   // шаг сетки стрелок
	testStep        = 2.0  // шаг пробного заряда вдоль поля
	bgScale         = 0.03 // масштаб для яркости фона по модулю поля
)

var (
	halfW = float64(screenWidth) / 2
	halfH = float64(screenHeight) / 2
)

type Charge struct {
	X, Y float64
	Q    float64
}

type Vec2 struct {
	X, Y float64
}

type Particle struct {
	X, Y float64
	Live bool
}

type Game struct {
	charges []Charge

	fieldLines [][]Vec2

	bgImage *ebiten.Image
	dirty   bool

	lastLeft  bool
	lastRight bool

	testParticle Particle
}

func NewGame() *Game {
	g := &Game{}

	g.charges = []Charge{
		{X: -150, Y: 0, Q: +1},
		{X: +150, Y: 0, Q: -1},
	}

	g.dirty = true
	return g
}

// Математика поля

func (g *Game) fieldAt(x, y float64) (float64, float64) {
	var Ex, Ey float64
	for _, c := range g.charges {
		dx := x - c.X
		dy := y - c.Y

		r2 := dx*dx + dy*dy
		if r2 < minR2 {
			r2 = minR2
		}
		r := math.Sqrt(r2)

		factor := kConst * c.Q / (r2 * r) // k*q/r^3

		Ex += factor * dx
		Ey += factor * dy
	}
	return Ex, Ey
}

func (g *Game) traceFieldLine(startX, startY float64, dir float64) []Vec2 {
	x := startX
	y := startY

	points := make([]Vec2, 0, 256)

	for i := 0; i < fieldLineMaxLen; i++ {
		Ex, Ey := g.fieldAt(x, y)
		E := math.Hypot(Ex, Ey)
		if E < 1e-6 {
			break
		}

		vx := Ex / E * dir
		vy := Ey / E * dir

		x += vx * fieldLineStep
		y += vy * fieldLineStep

		if x < -halfW-50 || x > halfW+50 || y < -halfH-50 || y > halfH+50 {
			break
		}

		nearCharge := false
		for _, c := range g.charges {
			if math.Hypot(x-c.X, y-c.Y) < seedRadius {
				nearCharge = true
				break
			}
		}
		if nearCharge {
			break
		}

		points = append(points, Vec2{X: x, Y: y})
	}

	return points
}

func (g *Game) recomputeFieldLines() {
	g.fieldLines = nil

	if len(g.charges) == 0 {
		return
	}

	for _, c := range g.charges {
		for i := 0; i < seedsPerCharge; i++ {
			angle := 2 * math.Pi * float64(i) / float64(seedsPerCharge)

			sx := c.X + seedRadius*math.Cos(angle)
			sy := c.Y + seedRadius*math.Sin(angle)

			dir := 1.0
			if c.Q < 0 {
				dir = -1.0
			}

			line := g.traceFieldLine(sx, sy, dir)
			if len(line) > 1 {
				g.fieldLines = append(g.fieldLines, line)
			}
		}
	}
}

func (g *Game) recomputeBackground() {
	img := ebiten.NewImage(screenWidth, screenHeight)

	for py := 0; py < screenHeight; py++ {
		y := float64(py) - halfH
		for px := 0; px < screenWidth; px++ {
			x := float64(px) - halfW

			Ex, Ey := g.fieldAt(x, y)
			E := math.Hypot(Ex, Ey)

			val := E * bgScale
			if val > 1 {
				val = 1
			}

			c := uint8(val * 255)
			img.Set(px, py, color.RGBA{c, c, c, 255})
		}
	}

	g.bgImage = img
}

func (g *Game) recomputeAll() {
	g.recomputeFieldLines()
	g.recomputeBackground()
	g.dirty = false
}

// Логика
func (g *Game) addChargeFromMouse(q float64) {
	x, y := ebiten.CursorPosition()
	wx := float64(x) - halfW
	wy := float64(y) - halfH

	g.charges = append(g.charges, Charge{X: wx, Y: wy, Q: q})
	g.dirty = true
}

func (g *Game) spawnTestParticleAtMouse() {
	x, y := ebiten.CursorPosition()
	wx := float64(x) - halfW
	wy := float64(y) - halfH

	g.testParticle = Particle{
		X:    wx,
		Y:    wy,
		Live: true,
	}
}

func (g *Game) updateTestParticle() {
	if !g.testParticle.Live {
		return
	}

	p := &g.testParticle

	Ex, Ey := g.fieldAt(p.X, p.Y)
	E := math.Hypot(Ex, Ey)
	if E < 1e-4 {
		return
	}

	vx := Ex / E
	vy := Ey / E

	p.X += vx * testStep
	p.Y += vy * testStep

	if math.Abs(p.X) > halfW+100 || math.Abs(p.Y) > halfH+100 {
		p.Live = false
	}
}

// Интерфейс

func (g *Game) Update() error {
	leftNow := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	rightNow := ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)

	if leftNow && !g.lastLeft {
		g.addChargeFromMouse(+1)
	}
	if rightNow && !g.lastRight {
		g.addChargeFromMouse(-1)
	}

	g.lastLeft = leftNow
	g.lastRight = rightNow

	if inpututil.IsKeyJustPressed(ebiten.KeyT) {
		g.spawnTestParticleAtMouse()
	}

	if g.dirty {
		g.recomputeAll()
	}

	g.updateTestParticle()

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.bgImage != nil {
		screen.DrawImage(g.bgImage, nil)
	} else {
		screen.Fill(color.RGBA{0, 0, 0, 255})
	}

	for _, line := range g.fieldLines {
		for i := 0; i < len(line)-1; i++ {
			x1 := float32(line[i].X + halfW)
			y1 := float32(line[i].Y + halfH)
			x2 := float32(line[i+1].X + halfW)
			y2 := float32(line[i+1].Y + halfH)

			vector.StrokeLine(
				screen,
				x1, y1, x2, y2,
				1,
				color.RGBA{255, 255, 255, 180},
				false,
			)
		}
	}

	for py := arrowGridStep / 2; py < screenHeight; py += arrowGridStep {
		for px := arrowGridStep / 2; px < screenWidth; px += arrowGridStep {
			x := float64(px) - halfW
			y := float64(py) - halfH

			Ex, Ey := g.fieldAt(x, y)
			E := math.Hypot(Ex, Ey)
			if E < 1e-3 {
				continue
			}

			scale := 15.0 / E
			dx := Ex * scale
			dy := Ey * scale

			x1 := float32(px)
			y1 := float32(py)
			x2 := float32(float64(px) + dx)
			y2 := float32(float64(py) + dy)

			col := color.RGBA{0, 255, 0, 200}
			vector.StrokeLine(screen, x1, y1, x2, y2, 1, col, false)

			angle := math.Atan2(float64(y2-y1), float64(x2-x1))
			headAngle1 := angle + 0.6
			headAngle2 := angle - 0.6
			headLen := 6.0

			hx1 := x2 - float32(headLen*math.Cos(headAngle1))
			hy1 := y2 - float32(headLen*math.Sin(headAngle1))
			hx2 := x2 - float32(headLen*math.Cos(headAngle2))
			hy2 := y2 - float32(headLen*math.Sin(headAngle2))

			vector.StrokeLine(screen, x2, y2, hx1, hy1, 1, col, false)
			vector.StrokeLine(screen, x2, y2, hx2, hy2, 1, col, false)
		}
	}

	for _, c := range g.charges {
		px := float32(c.X + halfW)
		py := float32(c.Y + halfH)

		col := color.RGBA{255, 80, 80, 255}
		if c.Q < 0 {
			col = color.RGBA{80, 80, 255, 255}
		}

		vector.DrawFilledCircle(screen, px, py, 7, col, false)
	}

	if g.testParticle.Live {
		px := float32(g.testParticle.X + halfW)
		py := float32(g.testParticle.Y + halfH)
		vector.DrawFilledCircle(screen, px, py, 4, color.RGBA{255, 255, 0, 255}, false)
	}

	face := basicfont.Face7x13
	text.Draw(screen, "Left click: + charge, Right click: - charge, T: test charge", face, 10, 20, color.White)
	text.Draw(screen, "Red: +q, Blue: -q, Yellow: test charge", face, 10, 40, color.White)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	fmt.Println("EBITEN STARTED")

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Поле точечных зарядов")

	game := NewGame()

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
