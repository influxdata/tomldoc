## Overview

TOMLDoc generates sample toml documents from specified structures type
declarations. It can be executed as a standalone executable or as a go-generate
command. Since this generates the toml document (and not the code to generate
the document), some restrictions on what can be generated apply. For instance,
interfaces are not supported as the concrete type cannot (for obvious reasons)
be resolved from source code or at compile-time.

This application supports multiple forms of structure and field type
declarations. For the purposes of this overview, we'll categorize these into
two coarse-grain groups. "Basic" structures are those whose fields do not
contain another struct. This generates a "flat" tomldoc:

Source (target: Vec3):
```go
////
//// Vec3
////
type Vec3 struct {
  // x component
  // !td:unc x = 0.0
  X float32 `toml:"x"`

  // y component
  // !td:unc y = 0.0
  Y float32 `toml:"y"`

  // z component
  // !td:unc z = 0.0
  Z float32 `toml:"z"`
}
```

Result:
```toml
##
## Vec3
##

# x component
x = 0.0

# y component
y = 0.0

# z component
z = 0.0
```

"Complex" structures are those whose fields contain another struct. By default
TOMLDoc does not "follow" struct declarations. This helps alleviate situations
where external structures should *NOT* be included in the document.
"!td:follow" must be specified for TOMLDoc to "follow" struct declarations.

Source (target: Baz):
```go
////
//// Foo
////
type Foo struct {
  // Bar
  // !td:unc bar = 0
  Bar int `toml:"bar"`
}

////
//// Baz
////
type Baz struct {
  // !td:follow
  Foo Foo
}
```

Result:
```toml
##
## Baz
##

[Foo]
  ##
  ## Foo
  ##

  # Bar
  bar = 0
```

TOMLDoc supports nested anonymous structs and anonymous fields. This
application follows the same visibility settings as "BurntSushi/toml".
Therefore, all fields must be exported (field name must start with a
capital letter) to appear in the toml document. Only structs with
exported members will appear for anonymous fields.

Source (target: Foo):
```go
////
//// Delta
////
type Delta struct {
  // D = <unix-timestamp>
  D uint64
}

////
//// Foo
////
type Foo struct {
  // Bar
  // !td:unc bar= 0
  Bar int `toml:"bar"`

  // Baz (not-exported)
  baz int

  // Bat
  Bat struct {
    // x component
    // !td:unc x = 0.0
    X float32 `toml:"x"`

    // y component
    // !td:unc y = 0.0
    Y float32 `toml:"y"`

    // z component
    // !td:unc z = 0.0
    Z float32 `toml:"z"`
  }

  // !td:follow
  Delta
}
```

Result:
```
##
## Foo
##

# Bar
bar = 0

# Bat
[Bat]
  # x component
  x = 0.0

  # y component
  y = 0.0

  # z component
  z = 0.0

# D = <unix-timestamp>
```

For instructions on how to run `tomldoc`, build then execute `tomldoc` with no
arguments.
