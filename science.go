/*
Copyright (C) 2013-2014 Regents of the University of Minnesota.
This file is part of InMAP.

InMAP is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

InMAP is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with InMAP.  If not, see <http://www.gnu.org/licenses/>.
*/

package inmap

import "github.com/ctessum/atmos/advect"

// Mixing calculates vertical mixing based on Pleim (2007), which is
// combined local-nonlocal closure scheme, for
// boundary layer and based on Wilson (2004) for above the boundary layer.
// Also calculate horizontal mixing.
func (c *Cell) Mixing(Δt float64) {
	c.Lock()
	for ii := range c.Cf {
		// Pleim (2007) Equation 10.
		for i, g := range c.GroundLevel { // Upward convection
			c.Cf[ii] += c.M2u * g.Ci[ii] * Δt * c.GroundLevelFrac[i]
		}
		for i, a := range c.Above {
			// Convection balancing downward mixing
			c.Cf[ii] += (a.M2d*a.Ci[ii]*a.Dz/c.Dz - c.M2d*c.Ci[ii]) *
				Δt * c.AboveFrac[i]
			// Mixing with above
			c.Cf[ii] += 1. / c.Dz * (c.KzzAbove[i] * (a.Ci[ii] - c.Ci[ii]) /
				c.DzPlusHalf[i]) * Δt * c.AboveFrac[i]
		}
		for i, b := range c.Below { // Mixing with below
			c.Cf[ii] += 1. / c.Dz * (c.KzzBelow[i] * (b.Ci[ii] - c.Ci[ii]) /
				c.DzMinusHalf[i]) * Δt * c.BelowFrac[i]
		}
		// Horizontal mixing
		for i, w := range c.West { // Mixing with West
			c.Cf[ii] += 1. / c.Dx * (c.KxxWest[i] *
				(w.Ci[ii] - c.Ci[ii]) / c.DxMinusHalf[i]) * Δt * c.WestFrac[i]
		}
		for i, e := range c.East { // Mixing with East
			c.Cf[ii] += 1. / c.Dx * (c.KxxEast[i] *
				(e.Ci[ii] - c.Ci[ii]) / c.DxPlusHalf[i]) * Δt * c.EastFrac[i]
		}
		for i, s := range c.South { // Mixing with South
			c.Cf[ii] += 1. / c.Dy * (c.KyySouth[i] *
				(s.Ci[ii] - c.Ci[ii]) / c.DyMinusHalf[i]) * Δt * c.SouthFrac[i]
		}
		for i, n := range c.North { // Mixing with North
			c.Cf[ii] += 1. / c.Dy * (c.KyyNorth[i] *
				(n.Ci[ii] - c.Ci[ii]) / c.DyPlusHalf[i]) * Δt * c.NorthFrac[i]
		}
	}
	c.Unlock()
}

// UpwindAdvection calculates advection in the cell based
// on the upwind differences scheme.
func (c *Cell) UpwindAdvection(Δt float64) {
	for ii := range c.Cf {
		for i, w := range c.West {
			flux := advect.UpwindFlux(c.UAvg, w.Ci[ii], c.Ci[ii], c.Dx) *
				c.WestFrac[i] * Δt
			massTransfer(c, w, ii, flux)
		}

		// Only calculate east advection when there is a boundary. Otherwise
		// east advection is calculated as west advection for another cell.
		for i, e := range c.East {
			if e.Boundary {
				flux := advect.UpwindFlux(e.UAvg, c.Ci[ii], e.Ci[ii], c.Dx) *
					c.EastFrac[i] * Δt
				massTransfer(c, e, ii, -flux)
			}
		}

		for i, s := range c.South {
			flux := advect.UpwindFlux(c.VAvg, s.Ci[ii], c.Ci[ii], c.Dy) *
				c.SouthFrac[i] * Δt
			massTransfer(c, s, ii, flux)
		}

		// Only calculate north advection when there is a boundary. Otherwise
		// north advection is calculated as south advection for another cell.
		for i, n := range c.North {
			if n.Boundary {
				flux := advect.UpwindFlux(n.VAvg, c.Ci[ii], n.Ci[ii], c.Dy) *
					c.NorthFrac[i] * Δt
				massTransfer(c, n, ii, -flux)
			}
		}

		for i, b := range c.Below {
			if c.Layer > 0 {
				flux := advect.UpwindFlux(c.WAvg, b.Ci[ii], c.Ci[ii], c.Dz) *
					c.BelowFrac[i] * Δt
				massTransfer(c, b, ii, flux)
			}
		}

		// Only calculate above advection when there is a boundary. Otherwise
		// above advection is calculated as below advection for another cell.
		for i, a := range c.Above {
			if a.Boundary {
				flux := advect.UpwindFlux(a.WAvg, c.Ci[ii], a.Ci[ii], c.Dz) *
					c.AboveFrac[i] * Δt
				massTransfer(c, a, ii, -flux)
			}
		}
	}
}

// massTransfer transfers mass between two cells, where flux is the
// concentration leaving c1 and pi is the pollutant index.
func massTransfer(c1, c2 *Cell, pi int, flux float64) {
	c1.Lock()
	c1.Cf[pi] += flux
	c1.Unlock()
	c2.Lock()
	c2.Cf[pi] -= flux * c1.Volume / c2.Volume
	c2.Unlock()
}

// MeanderMixing calculates changes in concentrations caused by meanders:
// adevection that is resolved by the underlying comprehensive chemical
// transport model but is not resolved by InMAP. It assumes that mass is evenly
// distributed among the cells within the previously calculated deviation distance.
func (c *Cell) MeanderMixing(Δt float64) {
	for pi := range c.Ci {
		for i, m := range c.EWMeanderCells {
			if c.Ci[pi] > m.Ci[pi] {
				flux := c.UDeviation * (m.Ci[pi] - c.Ci[pi]) / c.Dx * c.EWMeanderFrac[i] * Δt
				massTransfer(c, m, pi, flux)
			}
		}
		for i, m := range c.NSMeanderCells {
			if c.Ci[pi] > m.Ci[pi] {
				flux := c.VDeviation * (m.Ci[pi] - c.Ci[pi]) / c.Dy * c.NSMeanderFrac[i] * Δt
				massTransfer(c, m, pi, flux)
			}
		}
	}
}

const ammoniaFactor = 4.

// Chemistry calculates the secondary formation of PM2.5.
// Explicitely calculates formation of particulate sulfate
// from gaseous and aqueous SO2.
// Partitions organic matter ("gOrg" and "pOrg"), the
// nitrogen in nitrate ("gNO and pNO"), and the nitrogen in ammonia ("gNH" and
// "pNH) between gaseous and particulate phase
// based on the spatially explicit partioning present in the baseline data.
func (c *Cell) Chemistry(d *InMAPdata) {
	c.Lock()
	// All SO4 forms particles, so sulfur particle formation is limited by the
	// SO2 -> SO4 reaction.
	ΔS := c.SO2oxidation * c.Cf[igS] * d.Dt
	c.Cf[ipS] += ΔS
	c.Cf[igS] -= ΔS

	// NH3 / NH4 partitioning
	totalNH := c.Cf[igNH] + c.Cf[ipNH]
	// Caclulate difference from equilibrium particulate NH conc.
	eqNHpDistance := totalNH*c.NHPartitioning - c.Cf[ipNH]
	if c.Cf[igS] != 0. && eqNHpDistance > 0. { // particles will form
		// If ΔSOx is present and pNH4 concentration is below
		// equilibrium, assume that pNH4 formation
		// is limited by SO4 formation.
		ΔNH := min(max(ammoniaFactor*ΔS*mwN/mwS, 0.), eqNHpDistance)
		c.Cf[ipNH] += ΔNH
		c.Cf[igNH] -= ΔNH
	} else {
		// If pNH4 concentration is above equilibrium or if there is
		// no change in SOx present, assume instantaneous equilibration.
		c.Cf[ipNH] += eqNHpDistance
		c.Cf[igNH] -= eqNHpDistance
	}

	// NOx / pN0 partitioning
	totalNO := c.Cf[igNO] + c.Cf[ipNO]
	c.Cf[ipNO] = totalNO * c.NOPartitioning
	c.Cf[igNO] = totalNO * (1 - c.NOPartitioning)

	// VOC/SOA partitioning
	totalOrg := c.Cf[igOrg] + c.Cf[ipOrg]
	c.Cf[ipOrg] = totalOrg * c.AOrgPartitioning
	c.Cf[igOrg] = totalOrg * (1 - c.AOrgPartitioning)
	c.Unlock()
}

// DryDeposition calculates particle removal by dry deposition
func (c *Cell) DryDeposition(d *InMAPdata) {
	if c.Layer == 0 {
		fac := 1. / c.Dz * d.Dt
		noxfac := 1 - c.NOxDryDep*fac
		so2fac := 1 - c.SO2DryDep*fac
		vocfac := 1 - c.VOCDryDep*fac
		nh3fac := 1 - c.NH3DryDep*fac
		pm25fac := 1 - c.ParticleDryDep*fac
		c.Lock()
		c.Cf[igOrg] *= vocfac
		c.Cf[ipOrg] *= pm25fac
		c.Cf[iPM2_5] *= pm25fac
		c.Cf[igNH] *= nh3fac
		c.Cf[ipNH] *= pm25fac
		c.Cf[igS] *= so2fac
		c.Cf[ipS] *= pm25fac
		c.Cf[igNO] *= noxfac
		c.Cf[ipNO] *= pm25fac
		c.Unlock()
	}
}

// WetDeposition calculates particle removal by wet deposition
func (c *Cell) WetDeposition(Δt float64) {
	particleFrac := 1. - c.ParticleWetDep*Δt
	SO2Frac := 1. - c.SO2WetDep*Δt
	otherGasFrac := 1 - c.OtherGasWetDep*Δt
	c.Lock()
	c.Cf[igOrg] *= otherGasFrac
	c.Cf[ipOrg] *= particleFrac
	c.Cf[iPM2_5] *= particleFrac
	c.Cf[igNH] *= otherGasFrac
	c.Cf[ipNH] *= particleFrac
	c.Cf[igS] *= SO2Frac
	c.Cf[ipS] *= particleFrac
	c.Cf[igNO] *= otherGasFrac
	c.Cf[ipNO] *= particleFrac
	c.Unlock()
}

// convert float to int (rounding)
func f2i(f float64) int {
	return int(f + 0.5)
}

func max(vals ...float64) float64 {
	m := vals[0]
	for _, v := range vals {
		if v > m {
			m = v
		}
	}
	return m
}
func min(v1, v2 float64) float64 {
	if v1 < v2 {
		return v1
	}
	return v2
}
func amin(vals ...float64) float64 {
	m := vals[0]
	for _, v := range vals {
		if v < m {
			m = v
		}
	}
	return m
}
