use states_lamp::{Lamp, LampState};

#[test]
fn smash() {
    let mut l = Lamp::new(1);
    l.smash();
    assert_eq!(l.state(), LampState::Broken);
}
