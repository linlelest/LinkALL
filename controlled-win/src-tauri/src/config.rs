// 本地配置（服务器地址、locale），持久化到 SQLite kv 表
use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ServerConfig {
    pub public_url: String,
    pub official_server: String,
    pub ice_servers: Vec<IceServer>,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct IceServer {
    pub urls: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub username: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub credential: Option<String>,
}

impl ServerConfig {
    pub fn default_for(server_url: &str) -> Self {
        Self {
            public_url: server_url.to_string(),
            official_server: server_url.to_string(),
            ice_servers: vec![IceServer { urls: "stun:stun.l.google.com:19302".into(), username: None, credential: None }],
        }
    }
}
