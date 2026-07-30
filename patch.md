# patch.md

## meta

- name: runabot-addon-profile-tools
- target: MCPProxy Go fork
- version: 1.0.0

## commits

### 1. Add profile access and per-profile tool selection

#### background

The fork needs reusable authorization boundaries for agent credentials. A
single token's server and permission scope is too coarse when several agents
need different subsets of the same upstreams, so profiles provide shared
server/tool policy that can be assigned to multiple tokens.

#### files

##### internal/config/profiles.go (modify)

Define reusable access profiles that select upstream servers and optional
per-server MCP tool allowlists.

##### internal/profile/context.go (modify)

Resolve the effective profile scope for a request, including the union of all
profiles assigned to an agent token.

##### internal/auth/agent_token.go (modify)

Represent the profiles assigned to an agent token as the access_profiles list.

##### internal/httpapi/tokens.go (modify)

Expose access_profiles when creating, reading, and updating agent tokens, and
provide an endpoint to replace the assigned profile list without rotating the
token secret.

##### cmd/mcpproxy/profile_cmd.go (modify)

Add CLI commands for listing, showing, creating, updating, and deleting access
profiles.

##### cmd/mcpproxy/token_cmd.go (modify)

Add CLI support for assigning and replacing access profiles on agent tokens.

##### frontend/src/views/Profiles.vue (create)

Add the web UI for managing access profiles and their server/tool scopes.

##### frontend/src/views/AgentTokens.vue (modify)

Display access profiles and provide create/edit controls for agent-token
profiles and scopes.

##### frontend/src/services/api.ts (modify)

Add client methods for access-profile and agent-token management APIs.

#### verify

- Access profiles restrict discovery and tool calls to their configured scope.
- An agent token may have multiple access_profiles.
- Existing agent tokens can have their access profile list replaced without
  changing their secret.
- CLI and web UI expose access-profile management.

### 2. Add native mTLS listener for Runabot

#### background

The dashboard needs a native listener mode for deployments that require
transport encryption and client certificate authentication without placing a
separate TLS terminator in front of MCPProxy.

#### files

##### internal/config/config.go (modify)

Add configuration for the dashboard TLS listener and its client-certificate
authentication settings.

##### internal/config/loader.go (modify)

Load and validate the mTLS listener configuration, including certificate,
private-key, and client-CA paths.

##### internal/server/dashboard_tls.go (create)

Create the native HTTPS listener and configure mutual TLS verification for
clients presenting certificates signed by the configured client CA.

##### internal/server/server.go (modify)

Start and manage the dashboard mTLS listener alongside the existing server
lifecycle and shutdown paths.

##### docs/configuration.md (modify)

Document dashboard TLS and mTLS configuration options.

#### verify

- The dashboard can listen with server-side TLS.
- When client verification is enabled, only certificates signed by the
  configured client CA are accepted.
- Configuration hot reload and graceful shutdown handle the listener safely.
