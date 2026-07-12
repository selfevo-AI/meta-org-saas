use std::sync::Arc;

use tokio_postgres::{Client, NoTls};
use uuid::Uuid;

#[derive(Clone)]
pub struct PostgresNonceStore {
    client: Arc<Client>,
}

impl PostgresNonceStore {
    pub async fn connect(database_url: &str) -> Result<Self, tokio_postgres::Error> {
        let (client, connection) = tokio_postgres::connect(database_url, NoTls).await?;
        tokio::spawn(async move {
            if let Err(error) = connection.await {
                tracing::error!(%error, "security nonce database connection stopped");
            }
        });
        Ok(Self {
            client: Arc::new(client),
        })
    }

    pub async fn is_ready(&self) -> Result<bool, tokio_postgres::Error> {
        let row = self
            .client
            .query_one(
                r#"
                SELECT COALESCE(
                    has_table_privilege(
                        current_user,
                        to_regclass('platform.security_request_nonces'),
                        'INSERT,DELETE'
                    ),
                    FALSE
                )
                "#,
                &[],
            )
            .await?;
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
        let row = self
            .client
            .query_opt(
                r#"
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
                "#,
                &[&nonce, &request_timestamp_seconds, &expires_at_seconds],
            )
            .await?;
        Ok(row.is_some())
    }

    pub async fn delete_expired(&self) -> Result<u64, tokio_postgres::Error> {
        self.client
            .execute(
                "DELETE FROM platform.security_request_nonces WHERE expires_at < NOW()",
                &[],
            )
            .await
    }

    #[cfg(test)]
    pub fn client_for_test(&self) -> &Client {
        &self.client
    }

    #[cfg(test)]
    pub async fn delete_for_test(&self, nonce: &Uuid) -> Result<u64, tokio_postgres::Error> {
        self.client
            .execute(
                "DELETE FROM platform.security_request_nonces WHERE nonce = $1::uuid",
                &[&nonce],
            )
            .await
    }
}
