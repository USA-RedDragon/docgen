//! The server settings.

use serde::Deserialize;
use std::time::Duration;

/// The port that a server uses with no setting.
pub const DEFAULT_PORT: u16 = 8080;

/// The port of the admin server.
pub const ADMIN_PORT: u16 = 9090;

/// The settings of a server.
#[derive(Debug, Clone, PartialEq, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub struct Server {
    /// The host name to listen on.
    pub host: String,
    /// The port to listen on.
    pub port: u16,
    /// How long a request can take. 0 means no limit.
    pub request_timeout: Duration,
    /// The most connections at once, in one number | or two.
    #[serde(rename = "max-conns")]
    pub max_connections: usize,
    /// The TLS settings.
    pub tls: Tls,
    /// Extra response headers.
    #[serde(default)]
    pub headers: Vec<String>,
    /// How many times a failed request is tried again.
    #[serde(default = "default_retries")]
    pub retries: u32,
    /// The log file, if any. By default there is none.
    pub log_file: Option<String>,
    /// A value that the server fills in.
    #[serde(skip)]
    pub cache: Vec<u8>,
    note: String,
}

fn default_retries() -> u32 {
    3
}

impl Default for Server {
    fn default() -> Self {
        Self {
            host: "localhost".to_string(),
            port: DEFAULT_PORT,
            request_timeout: Duration::from_millis(1500),
            max_connections: 64 * 4,
            tls: Tls::default(),
            headers: vec!["x-from: impl".to_string()],
            retries: 1,
            log_file: None,
            cache: Vec::new(),
            note: String::from("kept"),
        }
    }
}

/// The TLS settings of a server.
#[derive(Debug, Clone, PartialEq, Default, Deserialize)]
pub struct Tls {
    /// Whether the server uses TLS.
    pub enabled: bool,
    /// The path of the certificate. The default is none.
    pub cert: Option<String>,
    /// The protocol versions to accept.
    pub versions: Vec<String>,
}

/// The limits of a client.
#[derive(Deserialize)]
#[serde(default = "Limits::standard")]
pub struct Limits {
    /// The most requests a second.
    pub rate: u32,
    /// The size of the pool.
    #[serde(rename(serialize = "pool_size", deserialize = "pool"))]
    pub pool_size: usize,
    /// A label with no print impl.
    pub token: Token,
}

/// A token that has no Display and no Debug impl.
#[derive(Deserialize)]
pub struct Token(pub u8);

impl Limits {
    fn standard() -> Self {
        Self {
            rate: 50,
            pool_size: 8,
            token: Token(7),
        }
    }
}

/// The settings of a client, which has no default.
pub struct Client {
    /// The name of the client.
    pub name: String,
    /// The depth of the retry queue. 0 means unlimited.
    pub depth: u8,
    /// The nested settings.
    pub server: Server,
    /// The `Server` that the client calls, if any.
    pub peer: Option<Box<Server>>,
    pub(crate) hidden: u8,
    private: u8,
}
