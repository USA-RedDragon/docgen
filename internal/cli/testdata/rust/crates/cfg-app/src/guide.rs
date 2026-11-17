//! The settings of the config guide.

use serde::Deserialize;
use std::time::Duration;

/// The port of a listener with no setting.
pub const LISTEN_PORT: u16 = 8000;

/// The settings of a listener.
#[derive(Debug, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub struct Listener {
    /// The host to listen on.
    pub host: String,
    /// The port to listen on.
    pub port: u16,
    /// How long a request can take. 0 means no limit.
    pub request_timeout: Duration,
    /// Extra response headers.
    #[serde(default)]
    pub headers: Vec<String>,
    /// How many times a failed request runs again.
    #[serde(default = "default_retries")]
    pub retries: u32,
}

fn default_retries() -> u32 {
    3
}

impl Default for Listener {
    fn default() -> Self {
        Self {
            host: "localhost".to_string(),
            port: LISTEN_PORT,
            request_timeout: Duration::from_millis(1500),
            headers: vec!["x-trace: on".to_string()],
            retries: 1,
        }
    }
}
