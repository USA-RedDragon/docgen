//! Machines that docgen:states records in part or not at all.
#![cfg_attr(feature = "forbid", forbid(missing_docs))]

/// A mode.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Mode {
    /// Idle.
    Idle,
    /// Busy.
    Busy,
}

/// A machine with two fields of an enum type and no state method.
pub struct Twin {
    /// The first mode.
    pub a: Mode,
    /// The second mode.
    pub b: Mode,
}

/// A machine whose state method returns no enum.
pub struct Count {
    n: u8,
}

impl Count {
    /// Returns the count.
    pub fn state(&self) -> u8 {
        self.n
    }

    /// Adds one.
    pub fn add(&mut self) {
        self.n += 1;
    }
}

/// A generic machine.
pub struct Boxed<T> {
    /// The mode.
    pub mode: Mode,
    /// The value.
    pub value: T,
}

/// A machine with no method to record.
pub struct Still {
    mode: Mode,
}

impl Still {
    /// Returns the mode.
    pub fn mode(&self) -> Mode {
        self.mode
    }
}

/// A view of a [`Worker`].
pub struct View<'a> {
    /// The mode of the worker.
    pub mode: &'a Mode,
}

/// A machine with methods that the hook does not record.
pub struct Worker {
    mode: Mode,
}

impl Worker {
    /// Returns an idle worker.
    pub fn new() -> Self {
        Worker { mode: Mode::Idle }
    }

    /// Returns a busy worker, or none.
    pub fn busy(yes: bool) -> Option<Self> {
        yes.then_some(Worker { mode: Mode::Busy })
    }

    /// Starts work.
    pub fn start(&mut self) {
        self.mode = Mode::Busy;
    }

    /// Starts work and returns a view of the worker.
    pub fn start_view(&mut self) -> View {
        self.mode = Mode::Busy;
        View { mode: &self.mode }
    }

    /// Stops work, later.
    pub async fn stop_later(&mut self) {
        self.mode = Mode::Idle;
    }

    /// Stops work.
    pub fn stop(mut self) -> Result<Self, Self> {
        if self.mode == Mode::Idle {
            return Err(self);
        }
        self.mode = Mode::Idle;
        Ok(self)
    }
}

impl Default for Worker {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn worker() {
        let mut w = Worker::new();
        w.start();
        assert_eq!(w.start_view().mode, &Mode::Busy);
        assert!(w.stop().is_ok());
        assert!(Worker::busy(true).is_some());
        let _ = Count { n: 0 }.state();
    }

    #[cfg(feature = "fail")]
    #[test]
    fn fails() {
        let mut w = Worker::new();
        w.start();
        panic!("the worker fails");
    }
}
