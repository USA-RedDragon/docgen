//! Errors of the app.

use std::num::ParseIntError;

/// An error of the app.
#[derive(Debug, thiserror::Error)]
pub enum Error {
    /// A file could not be read.
    #[error("read {path}: {source}")]
    Read {
        /// The path of the file.
        path: String,
        source: std::io::Error,
    },
    /// A number in the config is not valid. The line holds the number.
    #[error("line {line}: bad number")]
    Number {
        line: usize,
        #[source]
        cause: ParseIntError,
    },
    /// The job count is out of range.
    #[error(transparent)]
    Limit(#[from] crate::limits::LimitError),
    /// The app has no jobs | none at all.
    #[error("no jobs")]
    NoJobs,
}
