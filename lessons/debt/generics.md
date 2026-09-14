package main

import (
	"fmt"
	"math"
	"slices"
)

// Create and keep a list of Spheres
// Create a method to get Spheres ordered by susrface area
// Extend the logic to Squares and Rectangles keeping the functionality
// BAD - Generic | Interface
// make func (r Sphere) Scale(scale float64) Shape { to not loose the specialization
// variadic spread

type Sphere struct {
	Radius float64
}

var _ Shape = Sphere{}

type Rect struct {
	A float64
	B float64
}

var _ Shape = Rect{}

type RightTriangle struct {
	A float64
	B float64
}

var _ Shape = RightTriangle{}

var sphereStorage = []Shape{Sphere{1.0}, Sphere{3.0}, RightTriangle{4, 5}, Rect{1.0, 5.0}, Sphere{2.0}}

func (r Sphere) Area() float64 {
	return math.Pi * r.Radius * r.Radius
}
func (r Sphere) Scale(scale float64) Shape {
	return Sphere{Radius: r.Radius * scale}
}
func (r Rect) Area() float64 {
	return r.A * r.B
}
func (r Rect) Scale(scale float64) Shape {
	return Rect{A: r.A * scale, B: r.B * scale}
}
func (r RightTriangle) Area() float64 {
	return r.A * r.B / 2.0
}
func (r RightTriangle) Scale(scale float64) Shape {
	return Rect{A: r.A * scale, B: r.B * scale}
}

type Shape interface {
	Area() float64
	Scale(scale float64) Shape
}

func Add(s Shape) {
	sphereStorage = append(sphereStorage, s)
}

func Scale[T Shape](shape T, scale float64) T {
	return shape.Scale(scale).(T)
}

func Area[T Shape](shape T) float64 {
	return shape.Area()
}

func Get() []Shape {
	println(Scale(Sphere{1.0}, 5).Radius)

	sphereStorageClone := append([]Shape{}, sphereStorage...)
	slices.SortFunc(sphereStorageClone, func(s1, s2 Shape) int {
		if s1.Area() > s2.Area() {
			return 1
		}
		return -1
	})
	return sphereStorageClone
}

func main() {
	Add(Sphere{4.0})
	Add(Rect{4.0, 5.2})
	// Add(123)
	// Area(123)

	fmt.Printf("%v\n", sphereStorage)
	fmt.Printf("%v\n", Get())
}
