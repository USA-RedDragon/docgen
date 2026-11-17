//! A crate whose doctest fails.

/// Adds one.
///
/// ```
/// assert_eq!(run_c_broken::inc(1), 3);
/// ```
pub fn inc(n: u32) -> u32 {
    n + 1
}
