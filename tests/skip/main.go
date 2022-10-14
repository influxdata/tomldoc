package main

////
//// Foo
////
type Foo struct {
  // Bar
  // !td:unc Bar = 0.0
  Bar float32

  // Baz
  // !td:skip
  Baz float32
}
