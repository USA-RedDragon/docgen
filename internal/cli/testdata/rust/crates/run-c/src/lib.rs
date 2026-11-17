//! Helpers to run and to test.
//!
//! ```
//! assert_eq!(run_c::double(2), 4);
//! ```

/// Doubles a number.
///
/// ```
/// # let base = 21;
/// let n = run_c::double(base);
/// assert_eq!(n, 42);
/// ```
pub fn double(n: u32) -> u32 {
    n * 2
}

/// Greets a name.
pub fn greeting(name: &str) -> String {
    format!("Hello, {name}!")
}

/// Shapes.
pub mod shapes {
    /// The area of a rectangle.
    ///
    /// ```
    /// use run_c::shapes::area;
    ///
    /// assert_eq!(area(3, 4), 12);
    /// ```
    ///
    /// A square is a rectangle:
    ///
    /// ```
    /// assert_eq!(run_c::shapes::area(5, 5), 25);
    /// ```
    pub fn area(w: u32, h: u32) -> u32 {
        w * h
    }
}

/// A counter.
pub struct Counter(u32);

impl Counter {
    /// Starts at zero.
    pub fn new() -> Self {
        Counter(0)
    }

    /// Returns the next number.
    ///
    /// ```no_run
    /// let mut c = run_c::Counter::new();
    /// assert_eq!(c.next(), 1);
    /// ```
    #[allow(clippy::should_implement_trait)]
    pub fn next(&mut self) -> u32 {
        self.0 += 1;
        self.0
    }
}

impl Default for Counter {
    fn default() -> Self {
        Self::new()
    }
}

/// Has no example.
pub fn plain() {}

/// Shows code that nothing checks.
///
/// ```ignore
/// run_c::missing();
/// ```
pub fn unchecked() {}
