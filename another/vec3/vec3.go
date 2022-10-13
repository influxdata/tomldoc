package vec3

////
//// Vec3
////
type Vec3 struct {
	// x component
	// !td:unc x = 0.0
	X int `toml:"x"`

	// y component
	// !td:unc y = 0.0
	Y int `toml:"y"`

	// z component
	// !td:unc z = 0.0
	Z int `toml:"z"`

	// !td:follow
	D Delta
}

////
//// Delta
////
type Delta struct {
	// delta
	// d = <unix-epoch>
	D uint64 `toml:"d"`
}
