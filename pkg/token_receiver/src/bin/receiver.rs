use iroh::Endpoint;

#[tokio::main]
async fn main() -> std::result::Result<(), Box<dyn std::error::Error>> {
    let endpoint = Endpoint::builder()
        .alpns(vec![b"token-transfer".to_vec()])
        .discovery_n0()
        .bind()
        .await?;
        
    // In some iroh versions `endpoint.ticket()` requires a node ticket.
    // Let's use `endpoint.node_addr().await` to get the address and build a ticket.
    let addr = endpoint.node_addr().await?;
    let ticket = iroh_base::ticket::NodeTicket::new(addr);
    println!("Ticket: {}", ticket);
    
    // Accept one connection
    if let Some(incoming) = endpoint.accept().await {
        // incoming is a `Connecting`
        match incoming.await {
            Ok(connection) => {
                match connection.accept_bi().await {
                    Ok((mut send, mut recv)) => {
                        let mut code = Vec::new();
                        // read the code until EOF
                        let mut buf = [0u8; 1024];
                        if let Ok(Some(n)) = recv.read(&mut buf).await {
                            code.extend_from_slice(&buf[..n]);
                            if let Ok(code_str) = String::from_utf8(code) {
                                println!("RECEIVED CODE: {}", code_str);
                            }
                        }
                        
                        send.write_all(b"ACK").await?;
                        send.finish()?;
                    }
                    Err(e) => eprintln!("Failed to accept bi stream: {}", e),
                }
            }
            Err(e) => eprintln!("Failed to accept connection: {}", e),
        }
    }
    
    Ok(())
}
