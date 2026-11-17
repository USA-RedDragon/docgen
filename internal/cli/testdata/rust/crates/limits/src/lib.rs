//! Limits of the fixture.

use std::fmt;

/// The most jobs that run at once.
pub const MAX_JOBS: usize = 8;

/// The name of the fixture.
pub const NAME: &str = "limits";

/// The default timeout in seconds.
pub const TIMEOUT_SECS: u64 = MAX_JOBS as u64 * 5;

/// The names of the supported modes.
pub static MODES: [&str; 2] = ["fast", "slow"];

/// The error of a failed check.
#[derive(Debug, PartialEq, Eq)]
pub enum LimitError {
    /// The value is above the limit.
    TooLarge { value: usize, max: usize },
    /// The value is zero.
    Zero,
}

impl fmt::Display for LimitError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            LimitError::TooLarge { value, max } => write!(f, "{value} is above the limit {max}"),
            LimitError::Zero => f.write_str("the value is zero"),
        }
    }
}

impl std::error::Error for LimitError {}

/// Checks that jobs is between 1 and MAX_JOBS.
pub fn check_jobs(jobs: usize) -> Result<usize, LimitError> {
    match jobs {
        0 => Err(LimitError::Zero),
        n if n > MAX_JOBS => Err(LimitError::TooLarge { value: n, max: MAX_JOBS }),
        n => Ok(n),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rejects_zero() {
        assert_eq!(check_jobs(0), Err(LimitError::Zero));
    }
}
