package inmap

import (
	//"fmt"
	"math"
	"math/rand"
	"runtime"
	"testing"
)

var d *InMAPdata

const (
	testRow       = 25300 // somewhere in Chicago
	testTolerance = 1e-8
	Δt            = 6.   // seconds
	E             = 0.01 // emissions
)

func init() {
	runtime.GOMAXPROCS(8)
	d = InitInMAPdata("../wrf2aim/aimData_1km_50000/aimData_[layer].gob", 27, "8080")
	d.Dt = Δt
}

// Tests whether the cells correctly reference each other
func TestCellAlignment(t *testing.T) {
	for row, cell := range d.Data {
		if cell.Row != row {
			t.Logf("Failed for Row %v (layer %v) index", cell.Row, cell.Layer)
			t.FailNow()
		}
		for i, w := range cell.West {
			if len(w.East) != 0 {
				pass := false
				for j, e := range w.East {
					if e.Row == cell.Row {
						pass = true
						if different(w.KxxEast[j], cell.KxxWest[i]) {
							t.Logf("Kxx doesn't match")
							t.FailNow()
						}
						if different(w.DxPlusHalf[j], cell.DxMinusHalf[i]) {
							t.Logf("Dx doesn't match")
							t.FailNow()
						}
						break
					}
				}
				if !pass {
					t.Logf("Failed for Row %v (layer %v) West",
						cell.Row, cell.Layer)
					t.FailNow()
				}
			}
		}
		for i, e := range cell.East {
			if len(e.West) != 0 {
				pass := false
				for j, w := range e.West {
					if w.Row == cell.Row {
						pass = true
						if different(e.KxxWest[j], cell.KxxEast[i]) {
							t.Logf("Kxx doesn't match")
							t.FailNow()
						}
						if different(e.DxMinusHalf[j], cell.DxPlusHalf[i]) {
							t.Logf("Dx doesn't match")
							t.FailNow()
						}
						break
					}
				}
				if !pass {
					t.Logf("Failed for Row %v (layer %v) East",
						cell.Row, cell.Layer)
					t.FailNow()
				}
			}
		}
		for i, n := range cell.North {
			if len(n.South) != 0 {
				pass := false
				for j, s := range n.South {
					if s.Row == cell.Row {
						pass = true
						if different(n.KyySouth[j], cell.KyyNorth[i]) {
							t.Logf("Kyy doesn't match")
							t.FailNow()
						}
						if different(n.DyMinusHalf[j], cell.DyPlusHalf[i]) {
							t.Logf("Dy doesn't match")
							t.FailNow()
						}
						break
					}
				}
				if !pass {
					t.Logf("Failed for Row %v (layer %v) North",
						cell.Row, cell.Layer)
					t.FailNow()
				}
			}
		}
		for i, s := range cell.South {
			if len(s.North) != 0 {
				pass := false
				for j, n := range s.North {
					if n.Row == cell.Row {
						pass = true
						if different(s.KyyNorth[j], cell.KyySouth[i]) {
							t.Logf("Kyy doesn't match")
							t.FailNow()
						}
						if different(s.DyPlusHalf[j], cell.DyMinusHalf[i]) {
							t.Logf("Dy doesn't match")
							t.FailNow()
						}
						break
					}
				}
				if !pass {
					t.Logf("Failed for Row %v (layer %v) South",
						cell.Row, cell.Layer)
					t.FailNow()
				}
			}
		}
		for i, a := range cell.Above {
			if len(a.Below) != 0 {
				pass := false
				for j, b := range a.Below {
					if b.Row == cell.Row {
						pass = true
						if different(a.KzzBelow[j], cell.KzzAbove[i]) {
							t.Logf("Kzz doesn't match")
							t.FailNow()
						}
						if different(a.DzMinusHalf[j], cell.DzPlusHalf[i]) {
							t.Logf("Dz doesn't match")
							t.FailNow()
						}
						break
					}
				}
				if !pass {
					t.Logf("Failed for Row %v (layer %v) Above",
						cell.Row, cell.Layer)
					t.FailNow()
				}
			}
		}
		for i, b := range cell.Below {
			pass := false
			if cell.Layer == 0 && b.Row == cell.Row {
				pass = true
			} else if len(b.Above) != 0 {
				for j, a := range b.Above {
					if a.Row == cell.Row {
						pass = true
						if different(b.KzzAbove[j], cell.KzzBelow[i]) {
							t.Logf("Kzz doesn't match")
							t.FailNow()
						}
						if different(b.DzPlusHalf[j], cell.DzMinusHalf[i]) {
							t.Logf("Dz doesn't match")
							t.FailNow()
						}
						break
					}
				}
			} else {
				pass = true
			}
			if !pass {
				t.Logf("Failed for Row %v (layer %v) Below",
					cell.Row, cell.Layer)
				t.FailNow()
			}
		}
		// Assume upper cells are never higher resolution than lower cells
		for _, g := range cell.GroundLevel {
			g2 := g
			pass := false
			for {
				if len(g2.Above) == 0 {
					pass = false
					break
				}
				if g2.Row == g2.Above[0].Row {
					pass = false
					break
				}
				if g2.Row == cell.Row {
					pass = true
					break
				}
				g2 = g2.Above[0]
			}
			if !pass {
				t.Logf("Failed for Row %v (layer %v) GroundLevel",
					cell.Row, cell.Layer)
				t.FailNow()
			}
		}
	}
}

// Test whether the mixing mechanisms are properly conserving mass
func TestMixing(t *testing.T) {
	nsteps := 100
	for tt := 0; tt < nsteps; tt++ {
		d.Data[testRow].Ci[0] += E / d.Data[testRow].Dz // ground level emissions
		d.Data[testRow].Cf[0] += E / d.Data[testRow].Dz // ground level emissions
		for _, cell := range d.Data {
			cell.Mixing(Δt)
		}
		for _, cell := range d.Data {
			cell.Ci[0] = cell.Cf[0]
		}
	}
	sum := 0.
	maxval := 0.
	for _, cell := range d.Data {
		sum += cell.Cf[0] * cell.Dz
		maxval = max(maxval, cell.Cf[0])
	}
	t.Logf("sum=%.12g (it should equal %v)\n", sum, E*float64(nsteps))
	if different(sum, E*float64(nsteps)) {
		t.FailNow()
	}
	if !different(sum, maxval) {
		t.Log("All of the mass is in one cell--it didn't mix")
		t.FailNow()
	}
}

// Test whether mass is conserved during chemical reactions.
func TestChemistry(t *testing.T) {
	c := d.Data[testRow]
	nsteps := 10
	vals := make([]float64, len(polNames))
	for tt := 0; tt < nsteps; tt++ {
		sum := 0.
		for i := 0; i < len(vals); i++ {
			vals[i] = rand.Float64() * 10.
			sum += vals[i]
		}
		for i, v := range vals {
			c.Cf[i] = v
		}
		c.Chemistry(d)
		finalSum := 0.
		for _, val := range c.Cf {
			finalSum += val
			if val < 0 {
				chemPrint(t, vals, c)
				t.FailNow()
			}
		}
		if different(finalSum, sum) {
			t.FailNow()
		}
		//chemPrint(t, vals, c)
	}
}

func chemPrint(t *testing.T, vals []float64, c *Cell) {
	for i, val2 := range c.Cf {
		t.Logf("%v: initial=%.3g, final=%.3g\n", polNames[i], vals[i], val2)
	}
}

// Test whether mass is conserved during advection.
func TestAdvection(t *testing.T) {
	for _, c := range d.Data {
		c.Ci[0] = 0
		c.Cf[0] = 0
	}
	nsteps := 50
	for tt := 0; tt < nsteps; tt++ {
		c := d.Data[testRow]
		c.Ci[0] += E / c.Dz / c.Dy / c.Dx // ground level emissions
		c.Cf[0] += E / c.Dz / c.Dy / c.Dx // ground level emissions
		for _, c := range d.Data {
			c.UpwindAdvection(Δt)
		}
		for _, c := range d.Data {
			c.Ci[0] = c.Cf[0]
		}
	}
	sum := 0.
	layerSum := make(map[int]float64)
	for _, c := range d.Data {
		val := c.Cf[0] * c.Dy * c.Dx * c.Dz
		sum += val
		layerSum[c.Layer] += val
	}
	t.Logf("sum=%.12g (it should equal %v)\n", sum, E*float64(nsteps))
	if different(sum, E*float64(nsteps)) {
		t.FailNow()
	}
}

func different(a, b float64) bool {
	if math.Abs(a-b)/math.Abs(b) > testTolerance {
		return true
	} else {
		return false
	}
}