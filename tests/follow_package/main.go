package main

import (
  "github.com/influxdata/tomldoc/tests/vec3"
)

////
//// Mat3
////
type Mat3 struct {
  // !td:follow
  A vec3.Vec3

  // !td:follow
  B vec3.Vec3

  // !td:follow
  C vec3.Vec3
}
