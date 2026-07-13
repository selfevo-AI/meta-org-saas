use std::sync::Arc;

use tokio::sync::{Mutex, RwLock};
use tokio_postgres::{Client, NoTls};
use uuid::Uuid;

const READY_QUERY: &str = r#"
    SELECT COALESCE(
        has_table_privilege(
            current_user,
            to_regclass('platform.security_request_nonces'),
            'INSERT,DELETE'
        ),
        FALSE
    )
"#;

#[derive(Clone)]
pub struct PostgresNonceStore {
    inner: Arc<NonceStoreInner>,
}

struct NonceStoreInner {
    database_url: String,
    client: RwLock<Arc<Client>>,
    reconnect: Mutex<()>,
}

impl PostgresNonceStore {
    pub async fn connect(database_url: &str) -> Result<Self, tokio_postgres::Error> {
        let database_url = database_url.to_string();
        let client = connect_client(&database_url).await?;
        Ok(Self {
            inner: Arc::new(NonceStoreInner {
                database_url,
                client: RwLock::new(client),
                reconnect: Mutex::new(()),
            }),
        })
    }

    pub async fn is_ready(&self) -> Result<bool, tokio_postgres::Error> {
        let mut client = self.client().await?;
        let row = match client.query_one(READY_QUERY, &[]).await {
            Ok(row) => row,
            Err(error) if should_reconnect(&error, &client) => {
                tracing::warn!(%error, "security nonce readiness query lost database connection");
                client = self.reconnect(&client).await?;
                client.query_one(READY_QUERY, &[]).await?
            }
            Err(error) => return Err(error),
        };
        Ok(row.get(0))
    }

    pub async fn claim(
        &self,
        nonce: &Uuid,
        request_timestamp: i64,
        max_clock_skew_seconds: i64,
    ) -> Result<bool, tokio_postgres::Error> {
        let request_timestamp_seconds = request_timestamp as f64;
        let expires_at_seconds = request_timestamp
            .saturating_add(max_clock_skew_seconds)
            .saturating_add(1) as f64;
        let mut client = self.client().await?;
        let query = r#"
            INSERT INTO platform.security_request_nonces(
                nonce, request_timestamp, expires_at
            )
            VALUES (
                $1::uuid,
                to_timestamp($2),
                to_timestamp($3)
            )
            ON CONFLICT (nonce) DO NOTHING
            RETURNING nonce
        "#;
        let params: [&(dyn tokio_postgres::types::ToSql + Sync); 3] =
            [nonce, &request_timestamp_seconds, &expires_at_seconds];
        let row = match client.query_opt(query, &params).await {
            Ok(row) => row,
            Err(error) if should_reconnect(&error, &client) => {
                tracing::warn!(%error, "security nonce claim lost database connection");
                client = self.reconnect(&client).await?;
                client.query_opt(query, &params).await?
            }
            Err(error) => return Err(error),
        };
        Ok(row.is_some())
    }

    pub async fn delete_expired(&self) -> Result<u64, tokio_postgres::Error> {
        let query = "DELETE FROM platform.security_request_nonces WHERE expires_at < NOW()";
        let mut client = self.client().await?;
        match client.execute(query, &[]).await {
            Ok(deleted) => Ok(deleted),
            Err(error) if should_reconnect(&error, &client) => {
                tracing::warn!(%error, "security nonce cleanup lost database connection");
                client = self.reconnect(&client).await?;
                client.execute(query, &[]).await
            }
            Err(error) => Err(error),
        }
    }

    async fn client(&self) -> Result<Arc<Client>, tokio_postgres::Error> {
        let client = self.inner.client.read().await.clone();
        if !client.is_closed() {
            return Ok(client);
        }
        self.reconnect(&client).await
    }

    async fn reconnect(&self, failed: &Arc<Client>) -> Result<Arc<Client>, tokio_postgres::Error> {
        let _guard = self.inner.reconnect.lock().await;
        let current = self.inner.client.read().await.clone();
        if !Arc::ptr_eq(&current, failed) && !current.is_closed() {
            return Ok(current);
        }
        let client = connect_client(&self.inner.database_url).await?;
        *self.inner.client.write().await = client.clone();
        tracing::info!("security nonce database connection restored");
        Ok(client)
    }

    #[cfg(test)]
    pub async fn batch_execute_for_test(&self, query: &str) -> Result<(), tokio_postgres::Error> {
        self.client().await?.batch_execute(query).await
    }

    #[cfg(test)]
    pub async fn backend_pid_for_test(&self) -> Result<i32, tokio_postgres::Error> {
        let row = self
            .client()
            .await?
            .query_one("SELECT pg_backend_pid()", &[])
            .await?;
        Ok(row.get(0))
    }

    #[cfg(test)]
    pub async fn terminate_backend_for_test(
        &self,
        backend_pid: i32,
    ) -> Result<bool, tokio_postgres::Error> {
        let row = self
            .client()
            .await?
            .query_one("SELECT pg_terminate_backend($1)", &[&backend_pid])
            .await?;
        Ok(row.get(0))
    }

    #[cfg(test)]
    pub async fn delete_for_test(&self, nonce: &Uuid) -> Result<u64, tokio_postgres::Error> {
        self.client()
            .await?
            .execute(
                "DELETE FROM platform.security_request_nonces WHERE nonce = $1::uuid",
                &[nonce],
            )
            .await
    }
}

async fn connect_client(database_url: &str) -> Result<Arc<Client>, tokio_postgres::Error> {
    let (client, connection) = tokio_postgres::connect(database_url, NoTls).await?;
    tokio::spawn(async move {
        if let Err(error) = connection.await {
            tracing::error!(%error, "security nonce database connection stopped");
        }
    });
    Ok(Arc::new(client))
}

fn should_reconnect(error: &tokio_postgres::Error, client: &Client) -> bool {
    if client.is_closed() {
        return true;
    }
    error
        .code()
        .is_some_and(|code| matches!(code.code(), "57P01" | "57P02" | "57P03"))
}
