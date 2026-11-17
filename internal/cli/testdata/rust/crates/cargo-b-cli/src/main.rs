use clap::{Parser, Subcommand};

/// Inspect the demo workspace.
#[derive(Parser)]
#[command(name = "demo", version)]
struct Cli {
    /// How many jobs to run
    #[arg(short, long, default_value_t = 1)]
    jobs: u32,
    #[command(subcommand)]
    command: Command,
}

#[derive(Subcommand)]
enum Command {
    /// Print the jobs
    Show,
    /// Print the jobs with a prefix
    Label {
        /// Text to put before the jobs
        #[arg(long, default_value = "jobs")]
        prefix: String,
    },
}

fn main() {
    let cli = Cli::parse();
    let text = cargo_b_core::describe(cli.jobs);
    match cli.command {
        Command::Show => println!("{text}"),
        Command::Label { prefix } => println!("{prefix}: {text}"),
    }
}
