//! The error type of the store.

/// An error of the store.
#[derive(Debug, thiserror::Error)]
pub enum Error {
    /// The key is not in the store.
    #[error("key not found: {0}")]
    NotFound(String),
    /// Another process holds the lock of the store.
    #[error("store is locked")]
    Locked,
    /// The data on disk is corrupt.
    #[error("corrupt data")]
    Corrupt,
    /// The store is full. Nothing documents this one.
    #[error("store is full")]
    Full,
}

impl Error {
    /// Reads an error from its code.
    ///
    /// # Errors
    ///
    /// [`Self::Corrupt`] for a code that no variant has.
    pub fn from_code(code: u8) -> Result<Self> {
        match code {
            1 => Ok(Self::Locked),
            _ => Err(Self::Corrupt),
        }
    }
}

/// A result with the [`Error`] of the store.
pub type Result<T> = std::result::Result<T, Error>;
