use clap::{Parser, Subcommand};
use reqwest::{Url, blocking::Client, redirect::Policy};
use serde::Deserialize;
use std::{process::ExitCode, time::Duration};

#[derive(Parser)]
#[command(
    name = "loadout",
    version,
    about = "Local companion for the Loadout MCP gateway"
)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Check whether a Loadout server is reachable.
    Doctor {
        #[arg(long, default_value = "http://127.0.0.1:8787")]
        server: String,
    },
}

#[derive(Deserialize)]
struct Health {
    status: String,
    service: String,
    version: String,
}

fn doctor(server: &str) -> Result<Health, &'static str> {
    let mut url = Url::parse(server).map_err(|_| "server must be an HTTP(S) root URL")?;
    if !matches!(url.scheme(), "http" | "https")
        || url.host_str().is_none()
        || !url.username().is_empty()
        || url.password().is_some()
        || url.query().is_some()
        || url.fragment().is_some()
        || url.path() != "/"
    {
        return Err("server must be an HTTP(S) root URL without credentials, query or fragment");
    }
    url.set_path("/api/v1/health");
    let client = Client::builder()
        .timeout(Duration::from_secs(5))
        .redirect(Policy::none())
        .build()
        .map_err(|_| "could not initialize HTTP client")?;
    let response = client.get(url).send().map_err(|error| {
        if error.is_timeout() {
            "connection timed out; check the server and retry"
        } else {
            "could not connect; check the server and retry"
        }
    })?;
    if !response.status().is_success() {
        return Err("server returned an unsuccessful HTTP status");
    }
    let health: Health = response.json().map_err(|error| {
        if error.is_timeout() {
            "connection timed out; check the server and retry"
        } else {
            "server returned an invalid health response"
        }
    })?;
    if health.status != "ok"
        || health.service != "loadout"
        || health.version.trim().is_empty()
        || health.version.chars().any(char::is_control)
    {
        return Err("response is not a healthy Loadout service");
    }
    Ok(health)
}

fn main() -> ExitCode {
    let result = match Cli::parse().command {
        Commands::Doctor { server } => doctor(&server),
    };
    match result {
        Ok(health) => {
            println!("Loadout {} is reachable", health.version);
            ExitCode::SUCCESS
        }
        Err(message) => {
            eprintln!("loadout: {message}");
            ExitCode::FAILURE
        }
    }
}
