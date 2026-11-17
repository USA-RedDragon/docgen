/// An error of a [`Lamp`](crate::Lamp).
#[derive(Debug, thiserror::Error)]
pub enum LampError {
    /// The level is above the highest level.
    #[error("level {level} is above the highest level")]
    TooBright {
        /// The level that was asked for.
        level: u8,
    },
    /// The lamp is broken and gives no light again.
    #[error("the lamp is broken")]
    Broken,
}
