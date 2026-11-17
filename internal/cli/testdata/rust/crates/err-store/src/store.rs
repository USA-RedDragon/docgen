//! The store itself.

use crate::error::{Error, Result};

/// A store of values of one type. See [`Error`] for what can fail, and
/// [`crate::MAX_KEYS`] for the size.
pub struct Store<T> {
    /// The path of the store on disk.
    pub path: String,
    values: Vec<T>,
}

impl<T> Store<T> {
    /// The most values that a store holds, as [`MAX_KEYS`](crate::MAX_KEYS) says.
    pub const CAPACITY: usize = crate::MAX_KEYS;

    /// Opens the store at `path`.
    ///
    /// # Errors
    ///
    /// Fails with [`Error::Locked`] when another process holds the lock.
    pub fn open(path: &str) -> Result<Self> {
        let _ = path;
        Err(Error::Locked)
    }

    /// Reads the value of `index`.
    ///
    /// # Errors
    ///
    /// `NotFound` when there is no value at `index`.
    pub fn get(&self, index: usize) -> std::result::Result<&T, Error> {
        self.values.get(index).ok_or(Error::NotFound(index.to_string()))
    }

    /// Reads the value of `index`, or else the first one. It does not return
    /// [`Error::Locked`], and the section below names only `Self::NotFound`,
    /// which is not a variant of [`Error`] for a [`Store`].
    ///
    /// # Errors
    ///
    /// Returns `Self::NotFound` when the store is empty.
    ///
    /// # Panics
    ///
    /// Never panics, and does not return `Error::Full`.
    pub fn first(&self, index: usize) -> Result<&T> {
        self.values.get(index).or(self.values.first()).ok_or(Error::NotFound(String::new()))
    }

    /// Removes a key.
    ///
    /// # Errors
    ///
    /// - [`Error::NotFound`] when the key is missing.
    /// - `crate::error::Error::Locked` when another process holds the lock.
    ///
    /// ```text
    /// Error::Full is in a code block and does not count.
    /// ```
    pub fn remove(&mut self, key: &str) -> Result<T> {
        let _ = key;
        Err(Error::Locked)
    }

    /// Checks the lock. Not public, so no row lists it.
    ///
    /// # Errors
    ///
    /// [`Error::Locked`].
    pub(crate) fn check(&self) -> Result<()> {
        Ok(())
    }
}

/// Counts the keys of `store`.
///
/// # Errors
///
/// [`Error::Locked`] when the store is locked, and [`Error::NotFound`] for an
/// empty store.
pub fn count<T>(store: &Store<T>) -> Result<usize> {
    let _ = store;
    Err(Error::Locked)
}
