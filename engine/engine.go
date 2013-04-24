package engine

import (
	"code.google.com/p/mx3/cuda"
	"code.google.com/p/mx3/data"
	"code.google.com/p/mx3/util"
	cuda5 "github.com/barnex/cuda5/cuda"
	"log"
)

// User inputs
var (
	Aex     ScalFn        = Const(0)             // Exchange stiffness in J/m
	ExMask  StaggeredMask                        // Mask for exchange.
	Msat    ScalFn        = Const(0)             // Saturation magnetization in A/m
	Alpha   ScalFn        = Const(0)             // Damping constant
	B_ext   VecFn         = ConstVector(0, 0, 0) // External field in T
	DMI     ScalFn        = Const(0)             // Dzyaloshinskii-Moriya vector in J/m²
	Ku1     VecFn         = ConstVector(0, 0, 0) // Uniaxial anisotropy vector in J/m³
	Xi      ScalFn        = Const(0)
	SpinPol ScalFn        = Const(1)
	J       VecFn         = ConstVector(0, 0, 0)
)

// Accessible quantities
var (
	M       Settable // reduced magnetization output handle
	B_eff   Handle   // effective field (T) output handle
	Torque  Buffered // torque (?) output handle
	STT     Handle   // spin-transfer torque output handle
	B_demag Handle   // demag field (T) output handle
	B_dmi   Handle   // demag field (T) output handle
	B_exch  Handle   // exchange field (T) output handle
	B_uni   Handle   // field due to uniaxial anisotropy output handle
	Table   Handle   // output handle for tabular data (average magnetization etc.)
	Time    float64  // time in seconds  // todo: hide? setting breaks autosaves
	Solver  *cuda.Heun
)

// hidden quantities
var (
	mesh                             data.Mesh
	m, torque, b_eff, b_demag        *buffered // torque, b_eff, b_demag share storage!
	b_exch, b_ext, b_dmi, b_uni, stt *adder
	demag_                           *cuda.DemagConvolution
	vol                              *data.Slice
	postStep                         []func(m *data.Slice) // called on m after every step
)

func initialize() {

	// these 2 GPU arrays are re-used to stored various quantities.
	arr1, arr2 := cuda.NewSlice(3, &mesh), cuda.NewSlice(3, &mesh)

	// cell volumes currently unused
	vol = data.NilSlice(1, &mesh)

	// magnetization
	m = newBuffered(arr1, "m", nil)
	M = m

	// effective field
	b_eff = newBuffered(arr2, "B_eff", nil)
	B_eff = b_eff

	// demag field
	demag_ = cuda.NewDemag(&mesh)
	b_demag = newBuffered(arr2, "B_demag", func(b *data.Slice) {
		demag_.Exec(b, m.Slice, vol, Mu0*Msat()) //TODO: consistent msat or bsat
	})
	B_demag = b_demag

	// exchange field
	b_exch = newAdder(3, &mesh, "B_exch", func(dst *data.Slice) {
		cuda.AddExchange(dst, m.Slice, ExMask.mask, Aex(), Msat())
	})
	B_exch = b_exch

	// Dzyaloshinskii-Moriya field
	b_dmi = newAdder(3, &mesh, "B_dmi", func(dst *data.Slice) {
		d := DMI()
		if d != 0 {
			cuda.AddDMI(dst, m.Slice, d, Msat())
		}
	})
	B_dmi = b_dmi

	// uniaxial anisotropy
	b_uni = newAdder(3, &mesh, "B_uni", func(dst *data.Slice) {
		ku1 := Ku1() // in J/m3
		if ku1 != [3]float64{0, 0, 0} {
			cuda.AddUniaxialAnisotropy(dst, m.Slice, ku1[2], ku1[1], ku1[0], Msat())
		}
	})
	B_uni = b_uni

	// external field
	b_ext = newAdder(3, &mesh, "B_ext", func(dst *data.Slice) {
		bext := B_ext()
		cuda.AddConst(dst, float32(bext[2]), float32(bext[1]), float32(bext[0]))
	})

	// llg torque
	torque = newBuffered(arr2, "torque", func(b *data.Slice) {
		cuda.LLGTorque(b, m.Slice, b, float32(Alpha()))
	})
	Torque = torque

	// spin-transfer torque
	stt = newAdder(3, &mesh, "stt", func(dst *data.Slice) {
		j := J()
		if j != [3]float64{0, 0, 0} {
			p := SpinPol()
			jx := j[2] * p
			jy := j[1] * p
			jz := j[0] * p
			cuda.AddZhangLiTorque(dst, m.Slice, [3]float64{jx, jy, jz}, Msat(), nil, Alpha(), Xi())
		}
	})
	STT = stt

	// data table
	table := newTable("datatable")
	Table = table

	// solver
	torqueFn := func(good bool) *data.Slice {
		m.touch(good) // saves if needed
		table.send(m.Slice, good)
		b_demag.update(good)
		b_exch.addTo(b_eff.Slice, good)
		b_dmi.addTo(b_eff.Slice, good)
		b_uni.addTo(b_eff.Slice, good)
		b_ext.addTo(b_eff.Slice, good)
		b_eff.touch(good)
		torque.update(good)
		stt.addTo(torque.Slice, good)
		return torque.Slice
	}
	Solver = cuda.NewHeun(m.Slice, torqueFn, cuda.Normalize, 1e-15, Gamma0, &Time)
}

func PostStep(f func(m *data.Slice)) {
	postStep = append(postStep, f)
}

func step() {
	Solver.Step()

	for _, f := range postStep {
		f(m.Slice)
	}

	s := Solver
	util.Dashf("step: % 8d (%6d) t: % 12es Δt: % 12es ε:% 12e", s.NSteps, s.NUndone, Time, s.Dt_si, s.LastErr)
}

// injects arbitrary code into the engine run loops. Used by web interface.
var inject = make(chan func()) // inject function calls into the cuda main loop. Executed in between time steps.

// inject code into engine and wait for it to complete.
func injectAndWait(task func()) {
	ready := make(chan int)
	inject <- func() { task(); ready <- 1 }
	<-ready
}

// Run the simulation for a number of seconds.
func Run(seconds float64) {
	log.Println("run for", seconds, "s")
	stop := Time + seconds
	RunCond(func() bool { return Time < stop })
}

// Run the simulation for a number of steps.
func Steps(n int) {
	log.Println("run for", n, "steps")
	stop := Solver.NSteps + n
	RunCond(func() bool { return Solver.NSteps < stop })
}

// Runs as long as condition returns true.
func RunCond(condition func() bool) {
	checkInited() // todo: check in handler
	defer util.DashExit()

	pause = false
	for condition() && !pause {
		select {
		default:
			step()
		case f := <-inject:
			f()
		}
	}
	pause = true
}

// Enter interactive mode. Never returns.
func RunInteractive() {
	pause = true
	log.Println("entering interactive mode")
	if webPort == "" {
		GoServe(*Flag_port)
	}

	for {
		log.Println("awaiting interaction")
		f := <-inject
		f()
	}
}

// Set the magnetization to uniform state. // TODO: mv to settable
func SetMUniform(mx, my, mz float32) {
	checkInited()
	m.memset(mz, my, mx)
	m.normalize()
}

// Set magnetization from file.
func SetMFile(fname string) {
	util.FatalErr(setMFile(fname))
}

func setMFile(fname string) error {
	m, _, err := data.ReadFile(fname)
	if err != nil {
		return err
	}
	M.Upload(m)
	return nil
}

// Set the simulation mesh to Nx x Ny x Nz cells of given size.
// Can be set only once at the beginning of the simulation.
func SetMesh(Nx, Ny, Nz int, cellSizeX, cellSizeY, cellSizeZ float64) {
	var zeromesh data.Mesh
	if mesh != zeromesh {
		free()
	}
	if Nx <= 1 {
		log.Fatal("mesh size X should be > 1, have: ", Nx)
	}
	mesh = *data.NewMesh(Nz, Ny, Nx, cellSizeZ, cellSizeY, cellSizeX)
	log.Println("set mesh:", mesh.UserString())
	initialize()
}

func free() {
	log.Println("resetting gpu")
	cuda5.DeviceReset() // does not seem to clear allocations
	Init()
	dlQue = nil
}

func Mesh() *data.Mesh {
	checkInited()
	return &mesh
}

func checkInited() {
	if mesh.Size() == [3]int{0, 0, 0} {
		log.Fatal("need to set mesh first")
	}
}

// map of names to Handle does not work because Handles change on the fly
// *Handle does not work because we loose interfaceness.
func Quant(name string) (h Buffered, ok bool) {
	switch name {
	default:
		return nil, false
	case "m":
		return M, true
	case "torque":
		return Torque, true
	}
	return nil, false // rm for go 1.1
}
