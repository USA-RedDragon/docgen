fn main() {
    let jobs = std::env::args().nth(1).and_then(|a| a.parse().ok()).unwrap_or(1);
    match limits::check_jobs(jobs) {
        Ok(n) => println!("{n} jobs"),
        Err(e) => {
            eprintln!("error: {e}");
            std::process::exit(1);
        }
    }
}
