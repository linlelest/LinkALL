// 服务端 HTTP API 客户端
use anyhow::Result;
use reqwest::Client;
use serde::{Deserialize, Serialize};
use serde_json::Value;

#[derive(Clone)]
pub struct ServerClient {
    pub base: String,
    pub http: Client,
}

impl ServerClient {
    pub fn new(base: impl Into<String>) -> Self {
        Self {
            base: base.into().trim_end_matches('/').to_string(),
            http: Client::builder()
                .user_agent("LinkALL-Hosted/1.0")
                .build()
                .unwrap(),
        }
    }

    pub async fn get_config(&self) -> Result<Value> {
        Ok(self.http.get(format!("{}/api/config", self.base)).send().await?.json().await?)
    }

    pub async fn register(&self, body: Value) -> Result<Value> {
        Ok(self.http.post(format!("{}/api/devices/register", self.base)).json(&body).send().await?.json().await?)
    }

    pub async fn login(&self, body: Value) -> Result<Value> {
        Ok(self.http.post(format!("{}/api/devices/login", self.base)).json(&body).send().await?.json().await?)
    }

    pub async fn reset_code(&self, id: i64, token: &str, body: Value) -> Result<Value> {
        Ok(self.http
            .post(format!("{}/api/devices/{}/reset-code", self.base, id))
            .bearer_auth(token)
            .json(&body)
            .send()
            .await?
            .json()
            .await?)
    }

    pub async fn update_flags(&self, id: i64, token: &str, body: Value) -> Result<Value> {
        Ok(self.http
            .patch(format!("{}/api/devices/{}", self.base, id))
            .bearer_auth(token)
            .json(&body)
            .send()
            .await?
            .json()
            .await?)
    }
}

#[derive(Debug, Serialize, Deserialize)]
pub struct RegisterReq {
    pub device_code: String,
    pub device_password: String,
    pub name: Option<String>,
    pub platform: Option<String>,
    pub os_version: Option<String>,
    pub app_version: Option<String>,
    pub allow_anonymous: Option<bool>,
    pub require_device_code: Option<bool>,
    pub accept_connections: Option<bool>,
}
