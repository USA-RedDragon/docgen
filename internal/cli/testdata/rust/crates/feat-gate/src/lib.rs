//! Limits that depend on the Cargo features of the crate.

/// The size of every build.
pub const BASE: u32 = 8;

/// The size with the `big` feature.
///
/// ```
/// assert_eq!(feat_gate::BIG, 1024);
/// ```
#[cfg(feature = "big")]
pub const BIG: u32 = 1024;

/// Items of the `extra` feature.
#[cfg(feature = "extra")]
pub mod extra {
    /// The level with the `extra` feature.
    pub const LEVEL: u8 = 3;
}
