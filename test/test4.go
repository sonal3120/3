package main

import (
	. "nimble-cube/core"
	"nimble-cube/dump"
	"nimble-cube/gpu/conv"
	"nimble-cube/mag"
)

func main() {
	N0, N1, N2 := 1, 32, 128
	cx, cy, cz := 3e-9, 3.125e-9, 3.125e-9
	mesh := NewMesh(N0, N1, N2, cx, cy, cz)
	size := mesh.GridSize()

	m := MakeChan3(size)
	hd := MakeChan3(size)

	acc := 8
	kernel := mag.BruteKernel(mesh.ZeroPadded(), acc)
	go conv.NewSymmetricHtoD(size, kernel, m.MakeRChan3(), hd).Run()

	Msat := 1.0053
	aex := Mu0 * 13e-12 / Msat
	hex := MakeChan3(size)
	go mag.NewExchange6(m.MakeRChan3(), hex, mesh, aex).Run()

	heff := MakeChan3(size)
	go NewAdder3(heff, hd.MakeRChan3(), hex.MakeRChan3()).Run()

	const alpha = 1
	torque := MakeChan3(size)
	go mag.RunLLGTorque(torque, m.MakeRChan3(), heff.MakeRChan3(), alpha)

	const dt = 100e-15
	solver := mag.NewEuler(m, torque.MakeRChan3(), dt)
	mag.SetAll(m.UnsafeArray(), mag.Uniform(0, 0.1, 1))
	go dump.Autosave("m.dump", m.MakeRChan3(), 1000)

	solver.Steps(1000)
	// TODO: drain
	Cleanup()
}
