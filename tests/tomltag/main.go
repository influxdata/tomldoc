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
  Foo Foo `toml:"foo"`
}
