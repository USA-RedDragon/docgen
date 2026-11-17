//! Limits of the app.

use std::fmt;
use std::time::Duration;

// docgen:region max-jobs
/// The most jobs that run at once.
pub const MAX_JOBS: usize = 8;
// docgen:end

/// The jobs of a small machine.
pub const SMALL_JOBS: usize = MAX_JOBS / 4;

// docgen:region values
/// The name of the app.
pub const NAME: &str = "core-a";

/// The timeout of one job.
pub const TIMEOUT: Duration = Duration::from_millis(1500);

/// The names of the modes.
pub static MODES: [&str; 2] = ["fast", "slow"];

/// A network port.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Port(pub u16);

impl fmt::Display for Port {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "port {}", self.0)
    }
}

impl Port {
    /// The port of a plain HTTP server.
    pub const HTTP: Port = Port(80);
}

/// The port of the web server.
pub const PORT_WEB: Port = Port(8080);

/// The port that the app listens on.
pub static LISTEN: Port = PORT_WEB;

/// The tries of a job.
const RETRIES: u32 = 3;
// docgen:end

/// A type with no Display and no Debug.
pub struct Opaque;

/// A value with no Display and no Debug.
pub const OPAQUE: Opaque = Opaque;

/// The error of a job count.
#[derive(Debug, PartialEq, Eq, thiserror::Error)]
pub enum LimitError {
    /// The count is above MAX_JOBS.
    #[error("{0} is above the limit")]
    TooLarge(usize),
    /// The count is zero.
    #[error("the count is zero")]
    Zero,
}

// docgen:region check-jobs
/// Checks that jobs is between 1 and [`MAX_JOBS`].
///
/// ```
/// # use core_a::limits::check_jobs;
/// assert_eq!(check_jobs(2), Ok(2));
/// ```
pub fn check_jobs(jobs: usize) -> Result<usize, LimitError> {
    match jobs {
        0 => Err(LimitError::Zero),
        n if n > MAX_JOBS => Err(LimitError::TooLarge(n)),
        n => Ok(n.min(RETRIES as usize * MAX_JOBS)),
    }
}
// docgen:end
