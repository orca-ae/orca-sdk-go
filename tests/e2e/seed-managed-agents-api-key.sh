#!/usr/bin/env bash

set -euo pipefail

namespace=${1:?namespace is required}
deployment=${2:?registry deployment is required}
api_key=${3:?API key is required}

kubectl --namespace "${namespace}" exec -i "deployment/${deployment}" -- \
  env ORCA_E2E_PLAINTEXT_KEY="${api_key}" node --input-type=module - <<'NODE'
const argon2 = await import('argon2');
const { createHash } = await import('node:crypto');
const { Pool } = await import('pg');

const apiKey = process.env.ORCA_E2E_PLAINTEXT_KEY;
const databaseUrl = process.env.DATABASE_URL;
if (!apiKey || !databaseUrl) throw new Error('API key and DATABASE_URL are required');

const organizationId = 'org_orca_sdk_e2e';
const workspaceId = 'ws_orca_sdk_e2e';
const principal = 'orca-sdk-e2e';
const scopes = [
  'workspace.agents.create', 'workspace.agents.alter', 'workspace.agents.describe', 'workspace.agents.delete',
  'workspace.sessions.create', 'workspace.sessions.alter', 'workspace.sessions.describe', 'workspace.sessions.delete',
  'workspace.agentTriggers.create', 'workspace.agentTriggers.alter',
  'workspace.agentTriggers.describe', 'workspace.agentTriggers.delete',
  'workspace.environments.create', 'workspace.environments.alter', 'workspace.environments.describe', 'workspace.environments.delete',
  'workspace.files.create', 'workspace.files.alter', 'workspace.files.describe', 'workspace.files.delete',
];

const hashed = await argon2.hash(apiKey, { type: argon2.argon2id });
const fingerprint = createHash('sha256').update('orca-api-key\0').update(apiKey).digest('hex');
const pool = new Pool({ connectionString: databaseUrl, max: 1 });
try {
  await pool.query('BEGIN');
  await pool.query(
    `INSERT INTO organizations (id, name, status)
     VALUES ($1, $2, 'active')
     ON CONFLICT (id) DO UPDATE SET status = 'active', updated_at = now()`,
    [organizationId, 'Orca SDK E2E'],
  );
  await pool.query(
    `INSERT INTO workspaces (id, organization_id, name, status, created_by)
     VALUES ($1, $2, $3, 'active', $4)
     ON CONFLICT (id) DO UPDATE SET status = 'active', archived_at = NULL, updated_at = now()`,
    [workspaceId, organizationId, 'Orca SDK E2E', principal],
  );
  await pool.query(
    `INSERT INTO api_keys
       (id, workspace_id, hashed_key, key_fingerprint, principal, scopes, status, revoked_at)
     VALUES ($1, $2, $3, $4, $5, $6, 'active', NULL)
     ON CONFLICT (id) DO UPDATE SET
       hashed_key = EXCLUDED.hashed_key,
       key_fingerprint = EXCLUDED.key_fingerprint,
       principal = EXCLUDED.principal,
       scopes = EXCLUDED.scopes,
       status = 'active',
       revoked_at = NULL,
       updated_at = now()`,
    ['apikey_orca_sdk_e2e', workspaceId, hashed, fingerprint, principal, scopes],
  );
  await pool.query('COMMIT');
} catch (error) {
  await pool.query('ROLLBACK').catch(() => {});
  throw error;
} finally {
  await pool.end();
}
NODE
