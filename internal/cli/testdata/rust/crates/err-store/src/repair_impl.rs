use crate::error::Error;

/// Repairs the store at `path`.
///
/// # Errors
///
/// [`Error::Corrupt`] when the data cannot be repaired.
pub fn repair(path: &str) -> Result<(), crate::error::Error> {
    let _ = path;
    Err(Error::Corrupt)
}
