//! Telemetry/scan transport to the control plane.
//!
//! v0.1: plain HTTP/JSON POST. The production path adds mTLS and local
//! store-and-forward (retry on network failure).

use serde::Serialize;

/// POST any serialisable body as JSON. Returns the HTTP status on success.
pub fn post_json<T: Serialize>(url: &str, body: &T) -> Result<u16, String> {
    match ureq::post(url).send_json(body) {
        Ok(resp) => Ok(resp.status()),
        Err(ureq::Error::Status(code, _)) => Err(format!("server returned status {code}")),
        Err(e) => Err(format!("transport error: {e}")),
    }
}

/// GET a URL and decode the JSON body into T. Used by the agent to poll for jobs.
pub fn get_json<T: serde::de::DeserializeOwned>(url: &str) -> Result<T, String> {
    match ureq::get(url).call() {
        Ok(resp) => resp.into_json::<T>().map_err(|e| format!("decode error: {e}")),
        Err(ureq::Error::Status(code, _)) => Err(format!("server returned status {code}")),
        Err(e) => Err(format!("transport error: {e}")),
    }
}
