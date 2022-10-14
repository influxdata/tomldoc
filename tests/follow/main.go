package main

////
//// Foo
////
type Foo struct {
  // !td:unc F = "example"
  F string
}

////
//// Bar
////
type Bar struct {
  // !td:follow
  Foo Foo
}

////
//// Baz
////
type Baz struct {
  // !td:follow
  A Bar

  // !td:follow
  B *Bar

  // !td:follow
  C []Bar

  // !td:follow
  Bar
}
