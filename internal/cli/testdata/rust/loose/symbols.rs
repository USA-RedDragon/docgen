//! A file with no crate.
//! It holds one of each item.
#![allow(dead_code)]

/// A port.
/// It holds a number.
#[derive(Debug)]
/// A second doc block.
pub struct Port<T> {
    /// The number.
    number: T,
}

impl<T> Port<T> {
    /// Makes a port.
    #[inline]
    pub const fn new(number: T) -> Self {
        Port { number }
    }
}

impl std::fmt::Display for Port<u8> {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        write!(f, "{}", self.number)
    }
}

impl std::fmt::Debug for Port<u16> {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        write!(f, "{}", self.number)
    }
}

/// The nets.
pub mod net {
    //! Inside the nets.

    /// Connects.
    pub fn connect() {}
}

#[cfg(unix)]
fn dup() {}

#[cfg(windows)]
fn dup() {}

/// A shape of one line.
fn tiny() {}

macro_rules! twice {
    ($x:expr) => {
        $x * 2
    };
}

/// A shape.
pub trait Shape {
    /// The sides.
    const SIDES: u8;

    /// The area.
    fn area(&self) -> u32;
}

pub enum Color {
    Red,
    Green,
}

pub union Bits {
    int: u32,
    float: f32,
}

pub type Id = u64;

pub const LIMIT: u32 = 8;

pub static NAME: &str = "loose";

impl Shape for Port<u8> {
    const SIDES: u8 = 0;

    fn area(&self) -> u32 {
        0
    }
}

impl Shape for Port<u16> {
    const SIDES: u8 = 1;

    fn area(&self) -> u32 {
        1
    }
}

impl Color {
    pub fn name(&self) -> &str {
        "color"
    }
}
