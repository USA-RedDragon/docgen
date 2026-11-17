//! The configuration of the app.

use configulator::Config;

/// The configuration of the app.
#[derive(Config, Debug)]
pub struct AppConfig {
    #[configulator(name = "log-level", default = "info", description = "log level: debug, info, warn or error")]
    pub log_level: String,

    #[configulator(name = "jobs", short = 'j', default = "4", description = "jobs that run at once")]
    pub jobs: usize,

    #[configulator(name = "http", nested)]
    pub http: Http,

    #[configulator(name = "token", required, secret, description = "API token")]
    pub token: String,
}

/// The listener of the app.
#[derive(Config, Debug)]
pub struct Http {
    #[configulator(name = "host", default = "[::]", description = "listen address")]
    pub host: String,

    #[configulator(name = "port", env = "LISTEN_PORT", default = "8080", description = "listen port")]
    pub port: u16,
}
