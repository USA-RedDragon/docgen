fn main() {
    let mut names: Vec<String> = std::env::args().skip(1).collect();
    if names.is_empty() {
        names.push("world".to_string());
    }
    for name in names {
        println!("{}", run_c::greeting(&name));
    }
}
