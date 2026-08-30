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

### 3. Fix OAuth persistence and reverse-proxy login

#### background

OAuth settings changed through the management surface were not reliably
retained, and login/callback handling needed to work when the dashboard was
served through a reverse proxy. Persisting the settings and synchronizing the
UI closes both deployment gaps.

#### files

##### internal/oauth/config.go (create)

Persist OAuth configuration changes made through the management API so they
survive reloads and restarts.

##### internal/server/server.go (modify)

Wire OAuth configuration updates into the server and preserve login behavior
when MCPProxy is deployed behind a reverse proxy.

##### internal/httpapi/server.go (modify)

Expose the server-side OAuth update behavior through the management API.

##### frontend/src/views/ServerDetail.vue (modify)

Add the UI controls and feedback needed to update a server's OAuth settings.

##### frontend/src/stores/servers.ts (modify)

Keep the frontend server state synchronized after OAuth configuration changes.

#### verify

- OAuth configuration updates persist across server reloads.
- Reverse-proxy deployments retain the correct login and callback behavior.
- The server detail UI reflects successful OAuth updates.

### 4. Hide the telemetry notice when telemetry is disabled

#### background

Operators who disable telemetry—especially with DO_NOT_TRACK=1—should not be
shown a telemetry prompt or its opt-out disclosure. The notice must therefore
follow the server's effective telemetry state rather than an unrelated browser
dismissal flag.

#### files

##### internal/config/config.go (modify)

Expose DO_NOT_TRACK as part of the resolved telemetry setting returned to API
clients, keeping the web UI's effective state aligned with the runtime.

##### internal/contracts/converters.go (modify)

Always serialize the effective telemetry.enabled value, including environment
overrides when configuration explicitly enables telemetry.

##### internal/contracts/converters_test.go (modify)

Verify that DO_NOT_TRACK is reflected in the API configuration projection
without mutating the source configuration.

##### frontend/src/components/TelemetryBanner.vue (modify)

Load the resolved telemetry setting before rendering the notice. Keep the
notice hidden when telemetry is disabled in Settings, disabled by
DO_NOT_TRACK, or cannot be resolved.

##### frontend/tests/unit/telemetry-banner.spec.ts (create)

Verify that the notice is hidden for disabled telemetry and shown for enabled
telemetry.

#### verify

- DO_NOT_TRACK=1 causes the API's resolved telemetry.enabled value to be false.
- The web UI does not render the telemetry notice when telemetry.enabled is
  false.
- The notice remains available when telemetry.enabled is true.
