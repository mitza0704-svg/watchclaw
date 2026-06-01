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
