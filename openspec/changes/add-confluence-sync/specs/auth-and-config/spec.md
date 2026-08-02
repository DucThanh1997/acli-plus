## ADDED Requirements

### Requirement: Interactive site setup
The `acli-plus setup` command SHALL interactively collect a Confluence site URL, an Atlassian account email, and an API token, and store them on the machine keyed by the site host. The token input MUST NOT be echoed to the terminal.

**Acceptance Criteria**:
- setup run with a valid site/email/token → credentials are saved and a success message names the host
- token entry → characters are not displayed on screen
- setup re-run for an already-configured host → the existing entry is replaced, not duplicated
- entered credentials fail an authentication check → an actionable error is shown and nothing is saved

#### Scenario: First-time setup succeeds
- **WHEN** the user runs `acli-plus setup` and enters a valid site URL, email, and API token
- **THEN** the credentials are stored keyed by the site host and a confirmation naming the host is shown

#### Scenario: Token is entered hidden
- **WHEN** the user is prompted for the API token
- **THEN** the typed characters are not echoed to the terminal

#### Scenario: Re-running setup updates the same host
- **WHEN** the user runs `acli-plus setup` for a host that already has stored credentials
- **THEN** the stored entry for that host is overwritten and no duplicate entry is created

#### Scenario: Invalid credentials are rejected
- **WHEN** the entered email or token fails an authentication check against the site
- **THEN** an actionable error is shown and no credentials are written

### Requirement: Per-host credential store
acli-plus SHALL persist credentials for multiple Confluence sites simultaneously, keyed by host, in a machine-local store that is not world-readable (file permissions 0600). Tokens MUST NOT be written to any project-level configuration file.

**Acceptance Criteria**:
- two different sites configured → both tokens are retained and independently retrievable by host
- the credential store file is created or updated → its permissions are 0600
- a per-project config file is written → it contains no token field

#### Scenario: Multiple sites coexist
- **WHEN** the user runs setup for `acme.atlassian.net` and later for `clientb.atlassian.net`
- **THEN** both sites' credentials are stored and each can be retrieved by its host

#### Scenario: Credential file is not world-readable
- **WHEN** the credential store file is created or updated
- **THEN** its filesystem permissions are 0600

#### Scenario: Tokens never enter project config
- **WHEN** a per-project `acli-plus.yaml` is created
- **THEN** it contains site and space configuration but no API token

### Requirement: Per-project configuration
acli-plus SHALL support an optional, non-secret `acli-plus.yaml` in a project directory that declares the site and default space/parent, and this file SHALL be safe to commit to version control.

**Acceptance Criteria**:
- project config declares a default space → creating without a full URL uses that space
- no project config present → commands still work when a full URL is supplied
- project config present → it never contains secret material

#### Scenario: Project config supplies a default space
- **WHEN** `acli-plus.yaml` declares `space: DEV` and the user creates a page without specifying a space
- **THEN** the page is created in space `DEV`

#### Scenario: Commands work without project config
- **WHEN** no `acli-plus.yaml` exists and the user supplies a full page or parent URL
- **THEN** the command proceeds using the host and space parsed from the URL

### Requirement: Site and credential resolution
For each command, acli-plus SHALL determine the target Confluence host as follows: if the command supplies a URL containing a host, that host is authoritative; otherwise the host is taken from a `--site` flag, then an environment variable, then per-project config. The token SHALL be looked up from the per-host credential store using the resolved host. If no credentials exist for the resolved host, the command SHALL fail with guidance to run `acli-plus setup`.

**Acceptance Criteria**:
- a full URL is supplied → its host is used regardless of project config
- no URL host and project config sets `site` → the project config host is used
- the resolved host has no stored credentials → the command fails with a message pointing to `acli-plus setup`

#### Scenario: URL host is authoritative
- **WHEN** a command is given a page URL on `acme.atlassian.net` while project config lists a different site
- **THEN** the command targets `acme.atlassian.net` and uses that host's stored token

#### Scenario: Host falls back to project config
- **WHEN** a command supplies no URL host and `acli-plus.yaml` sets `site: https://acme.atlassian.net`
- **THEN** the command targets `acme.atlassian.net`

#### Scenario: Missing credentials give actionable guidance
- **WHEN** the resolved host has no entry in the credential store
- **THEN** the command fails and instructs the user to run `acli-plus setup`
