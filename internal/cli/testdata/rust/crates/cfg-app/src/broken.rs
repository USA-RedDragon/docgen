//! A struct whose Default panics.

/// A struct with a Default that panics.
pub struct Boom {
    /// The size.
    pub size: u8,
}

impl Default for Boom {
    fn default() -> Self {
        panic!("no default for Boom")
    }
}
