//! A dimmable lamp, the Rust twin of the Go package machine of this sample.
//!
//! # Levels
//!
//! The level of a [`Lamp`] runs from 0, off, to [`MAX_LEVEL`]. Each
//! [`Lamp::brighter`] adds [`STEP`], up to the top.

mod error;

pub use error::LampError;

/// The highest level of a lamp.
pub const MAX_LEVEL: u8 = 100;

/// The level that [`Lamp::brighter`] adds.
pub const STEP: u8 = MAX_LEVEL / 4;

/// A lamp with a level. A broken lamp keeps level 0.
#[derive(Debug, Default)]
pub struct Lamp {
    level: u8,
    broken: bool,
}

impl Lamp {
    /// Sets the level of the lamp.
    ///
    /// # Errors
    ///
    /// A level above [`MAX_LEVEL`] is [`LampError::TooBright`], and a broken
    /// lamp is [`LampError::Broken`].
    pub fn set(&mut self, level: u8) -> Result<(), LampError> {
        if self.broken {
            return Err(LampError::Broken);
        }
        if level > MAX_LEVEL {
            return Err(LampError::TooBright { level });
        }
        self.level = level;
        Ok(())
    }

    /// Adds [`STEP`] to the level, up to [`MAX_LEVEL`].
    ///
    /// # Errors
    ///
    /// A broken lamp is [`LampError::Broken`].
    pub fn brighter(&mut self) -> Result<(), LampError> {
        self.set(self.level.saturating_add(STEP).min(MAX_LEVEL))
    }

    /// Breaks the lamp for good.
    pub fn smash(&mut self) {
        self.level = 0;
        self.broken = true;
    }

    /// The level of the lamp.
    pub fn level(&self) -> u8 {
        self.level
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn steps_stop_at_the_top() {
        let mut lamp = Lamp::default();
        for _ in 0..10 {
            lamp.brighter().unwrap();
        }
        assert_eq!(lamp.level(), MAX_LEVEL);
        assert!(matches!(lamp.set(MAX_LEVEL + 1), Err(LampError::TooBright { .. })));
        lamp.smash();
        assert!(matches!(lamp.set(1), Err(LampError::Broken)));
    }
}
