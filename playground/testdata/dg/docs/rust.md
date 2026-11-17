# The Rust lamp

docgen reads the Rust crate in `crates/lamp` with a helper that parses its source. In the playground, the helper runs in the tab, so the markers below work. A marker that builds or runs Rust code, such as the value of a const without `print=raw`, needs cargo, which the playground does not have.

<!-- docgen:doc crate=lamp -->
A dimmable lamp, the Rust twin of the Go package machine of this sample.

### Levels

The level of a `Lamp` runs from 0, off, to `MAX_LEVEL`. Each
`Lamp::brighter` adds `STEP`, up to the top.
<!-- /docgen:doc -->

The highest level is <!-- docgen:const crate=lamp name=MAX_LEVEL print=raw -->`100`<!-- /docgen:const -->, and a step is <!-- docgen:const crate=lamp name=STEP print=raw -->`MAX_LEVEL / 4`<!-- /docgen:const -->.

## Set

<!-- docgen:doc crate=lamp name=Lamp::set -->
Sets the level of the lamp.

### Errors

A level above `MAX_LEVEL` is `LampError::TooBright`, and a broken
lamp is `LampError::Broken`.
<!-- /docgen:doc -->

<!-- docgen:code file=crates/lamp/src/lib.rs symbol=Lamp::set -->
```rust
pub fn set(&mut self, level: u8) -> Result<(), LampError> {
    if self.broken {
        return Err(LampError::Broken);
    }
    if level > MAX_LEVEL {
        return Err(LampError::TooBright { level });
    }
    self.level = level;
    Ok(())
}
```
<!-- /docgen:code -->

## Errors

<!-- docgen:errors crate=lamp type=LampError -->
| Error | Returned by | Message | Source | Meaning |
|---|---|---|---|---|
| `LampError::TooBright` | `Lamp::set` | `level {level} is above the highest level` |  | The level is above the highest level. |
| `LampError::Broken` | `Lamp::brighter`, `Lamp::set` | `the lamp is broken` |  | The lamp is broken and gives no light again. |
<!-- /docgen:errors -->
