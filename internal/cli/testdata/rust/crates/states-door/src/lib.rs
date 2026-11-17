//! A door whose state is the variant of an enum, for docgen:states.

/// A door. Its variant is its state.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Door {
    /// Shut and not locked.
    Closed,
    /// Open.
    Open,
    /// Shut and locked with a code.
    Locked {
        /// The code that unlocks the door.
        code: u16,
    },
}

/// An event that a [`Door`] takes.
#[derive(Clone, Copy, Debug)]
pub enum Event {
    /// A push on the door.
    Push,
    /// A pull on the door.
    Pull,
    /// A lock with a code.
    Lock(u16),
    /// An unlock with a code.
    Unlock(u16),
}

impl Door {
    /// Returns the door after the event.
    pub fn on(self, event: Event) -> Self {
        match (self, event) {
            (Door::Closed, Event::Push) => Door::Open,
            (Door::Open, Event::Pull) => Door::Closed,
            (Door::Closed, Event::Lock(code)) => Door::Locked { code },
            (Door::Locked { code }, Event::Unlock(c)) if c == code => Door::Closed,
            (door, _) => door,
        }
    }

    /// Opens the door when it is closed.
    pub fn open(&mut self) {
        if *self == Door::Closed {
            *self = Door::Open;
        }
    }

    /// Closes the door when it is open.
    pub fn close(&mut self) {
        if *self == Door::Open {
            *self = Door::Closed;
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn events() {
        let d = Door::Closed.on(Event::Push).on(Event::Pull);
        let d = d.on(Event::Lock(7)).on(Event::Unlock(7));
        assert_eq!(d, Door::Closed);
    }

    #[test]
    fn open_close() {
        let mut d = Door::Closed;
        d.open();
        d.close();
        assert_eq!(d, Door::Closed);
    }
}
