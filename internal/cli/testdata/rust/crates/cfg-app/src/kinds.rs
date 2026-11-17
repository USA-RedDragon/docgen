//! Types that docgen:config rejects.

/// A mode.
pub enum Mode {
    /// On.
    On,
    /// Off.
    Off,
}

/// A pair.
pub struct Pair(pub u8, pub u8);

/// A unit.
pub struct Nothing;

/// A wrapper.
pub struct Wrapper<T> {
    /// The value.
    pub value: T,
}

/// A struct with no public field and no Deserialize.
pub struct Hidden {
    value: u8,
}

/// The limit.
pub const LIMIT: u8 = 1;
