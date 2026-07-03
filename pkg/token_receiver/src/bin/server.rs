use axum::{
    extract::{Path, Query},
    response::{Redirect, Html},
    routing::get,
    Router,
};
use serde::Deserialize;
use std::env;
use std::str::FromStr;

#[derive(Deserialize)]
struct AuthQuery {
    code: String,
}

#[tokio::main]
async fn main() {
    let addr = env::var("SERVER_ADDR").unwrap_or_else(|_| "0.0.0.0:7070".to_string());

    let app = Router::new()
        .route("/auth/:ticket_str", get(auth_handler))
        .route("/render", get(render_handler));

    println!("Starting server on {}", addr);
    let listener = tokio::net::TcpListener::bind(&addr).await.unwrap();
    axum::serve(listener, app).await.unwrap();
}

async fn auth_handler(
    Path(ticket_str): Path<String>,
    Query(query): Query<AuthQuery>,
) -> Redirect {
    println!("Received auth call with ticket: {}", ticket_str);
    
    let success = match send_token_via_iroh(&ticket_str, &query.code).await {
        Ok(_) => {
            println!("Successfully sent token for ticket: {}", ticket_str);
            true
        },
        Err(e) => {
            eprintln!("Failed to send token for ticket {}: {}", ticket_str, e);
            false
        }
    };
    
    Redirect::to(&format!("/render?success={}", success))
}

#[derive(Deserialize)]
struct RenderQuery {
    success: bool,
}

async fn render_handler(Query(query): Query<RenderQuery>) -> Html<String> {
    let msg = if query.success {
        "The token was successfully sent."
    } else {
        "The token was not successfully sent."
    };
    
    Html(format!("<!DOCTYPE html><html><body><h1>{}</h1></body></html>", msg))
}

async fn send_token_via_iroh(ticket_str: &str, code: &str) -> std::result::Result<(), Box<dyn std::error::Error>> {
    println!("Parsing ticket...");
    let ticket = iroh_base::ticket::NodeTicket::from_str(ticket_str)?;
    
    println!("Starting local Iroh endpoint...");
    let endpoint = iroh::Endpoint::builder().discovery_n0().bind().await?;
    
    println!("Connecting to receiver node...");
    let connection = endpoint.connect(ticket.node_addr().clone(), b"token-transfer").await?;
    
    println!("Opening bidirectional stream...");
    let (mut send, mut recv) = connection.open_bi().await?;
    
    println!("Sending token to receiver...");
    send.write_all(code.as_bytes()).await?;
    send.finish()?;
    
    println!("Waiting for ACK from receiver...");
    let mut buf = [0u8; 3];
    recv.read_exact(&mut buf).await?;
    
    if &buf == b"ACK" {
        println!("Received ACK from receiver.");
        Ok(())
    } else {
        Err("invalid ack".into())
    }
}
