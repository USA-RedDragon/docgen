//! A store that returns errors.
//!
//! Open a [`Store`] and read a key with [`Store::get`]. Every failure is an
//! [`Error`], such as [`Error::NotFound`].

pub mod error;
pub mod store;

mod repair_impl;

pub use error::{Error, Result};
pub use repair_impl::repair;
pub use store::Store;

/// The most keys that a store holds.
pub const MAX_KEYS: usize = 1024;
