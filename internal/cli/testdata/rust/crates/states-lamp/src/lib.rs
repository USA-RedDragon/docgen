//! A lamp with a timer, for docgen:states.

use std::fmt;

/// The state of a [`Lamp`].
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum LampState {
    /// Gives no light.
    Off,
    /// Gives light until the wait ends.
    On,
    /// Never gives light again.
    Broken,
}

/// An error of a [`Lamp`] call.
#[derive(Debug, PartialEq, Eq)]
pub enum LampError {
    /// The lamp is on.
    Busy,
    /// The lamp is not on.
    Off,
}

impl fmt::Display for LampError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            LampError::Busy => f.write_str("the lamp is on"),
            LampError::Off => f.write_str("the lamp is not on"),
        }
    }
}

/// A lamp that turns off after a wait.
#[derive(Debug)]
pub struct Lamp {
    state: LampState,
    until: u64,
    wait: u64,
    name: String,
}

impl Lamp {
    /// Returns an off lamp that stays on for wait ticks.
    pub fn new(wait: u64) -> Self {
        Lamp {
            state: LampState::Off,
            until: 0,
            wait,
            name: String::from("lamp"),
        }
    }

    /// Returns the state of the lamp.
    pub fn state(&self) -> LampState {
        self.state
    }

    /// Turns the lamp on at the time now.
    pub fn start(&mut self, now: u64) -> Result<(), LampError> {
        if self.state == LampState::On {
            return Err(LampError::Busy);
        }
        if self.state == LampState::Off {
            self.state = LampState::On;
            self.until = now + self.wait;
        }
        Ok(())
    }

    /// Turns the lamp off.
    pub fn stop(&mut self) -> Result<(), LampError> {
        self.check_on()?;
        self.off();
        Ok(())
    }

    /// Turns the lamp off when its wait has ended at the time now.
    pub fn tick(&mut self, now: u64) {
        if self.state == LampState::On && now >= self.until {
            self.off();
        }
    }

    /// Breaks the lamp for good.
    pub fn smash(&mut self) {
        self.state = LampState::Broken;
    }

    /// Returns the name of the lamp, to change it.
    pub fn name_mut(&mut self) -> &mut String {
        &mut self.name
    }

    fn check_on(&self) -> Result<(), LampError> {
        if self.state == LampState::On {
            Ok(())
        } else {
            Err(LampError::Off)
        }
    }

    fn off(&mut self) {
        self.state = LampState::Off;
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn lamp() {
        let mut l = Lamp::new(3);
        assert_eq!(l.start(0), Ok(()));
        l.tick(3);
        assert_eq!(l.start(0), Ok(()));
        assert_eq!(l.stop(), Ok(()));
        assert_eq!(l.stop(), Err(LampError::Off));
        l.name_mut().push('!');
    }
}
