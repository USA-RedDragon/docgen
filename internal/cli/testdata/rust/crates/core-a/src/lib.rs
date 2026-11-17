// docgen:region crate-doc
//! The core of the fixture app.
//!
//! # Limits
//!
//! The app runs at most [`limits::MAX_JOBS`] jobs at once, and each job has a
//! timeout.
//!
//! ## Timeouts
//!
//! A job that runs past its timeout stops.
//!
//! # Other
//!
//! Not copied.
// docgen:end

pub mod config;
pub mod errors;
pub mod limits;
pub mod net;

#[path = "plain/renamed.rs"]
pub mod renamed;

/// Shapes of the fixture.
pub mod shapes {
    /// The sides of a square.
    pub const SQUARE_SIDES: u8 = 4;

    mod inner {
        /// The ratio of the sides.
        const RATIO: f64 = 1.5;
    }
}

/// Not in any build.
#[cfg(any())]
pub mod never {
    /// A value of no build.
    pub const NEVER: u8 = 0;
}

/// Only on Unix.
#[cfg(unix)]
pub mod unix;
