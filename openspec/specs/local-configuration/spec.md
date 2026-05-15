## Requirements

### Requirement: Initialize local storage directories
The system SHALL provide an `init` command that creates the local configuration and kubeconfig cache directories required by the CLI.

#### Scenario: Init creates required directories
- **WHEN** the user runs `k3ctx init`
- **THEN** the system creates the user data config directory and kubeconfig cache directory
- **AND** the command reports that `k3ctx` was initialized

### Requirement: Resolve effective configuration
The system SHALL resolve runtime configuration from default config file candidates and environment overrides.

#### Scenario: CONFIG_FILE overrides config path
- **WHEN** `CONFIG_FILE` is set
- **THEN** the system loads configuration from that path

#### Scenario: Default config candidates are searched
- **WHEN** `CONFIG_FILE` is not set
- **THEN** the system searches the XDG config path, project config path, and legacy home config path in order

#### Scenario: Environment variables override file values
- **WHEN** supported environment variables are set for configuration keys
- **THEN** the system uses those environment values in the effective configuration

### Requirement: Resolve local storage paths
The system SHALL use XDG-style local data paths for config and kubeconfig cache storage.

#### Scenario: XDG data home is honored
- **WHEN** `XDG_DATA_HOME` is set
- **THEN** the system resolves app data paths under that directory

#### Scenario: Default data home is used
- **WHEN** `XDG_DATA_HOME` is not set
- **THEN** the system resolves app data paths under `$HOME/.local/share/k3ctx`

#### Scenario: K3CTX_CONFIG_DIR overrides config directory
- **WHEN** `K3CTX_CONFIG_DIR` is set
- **THEN** the system resolves the config file as `config.yaml` inside that directory

### Requirement: Resolve inventory path
The system SHALL resolve the inventory directory from configured path, project-local inventory, ancestor `ansible/inventory`, or fallback path.

#### Scenario: Existing configured inventory path is used
- **WHEN** `inventory_path` resolves to an existing directory
- **THEN** the system uses that directory as the inventory path

#### Scenario: Project inventory is used
- **WHEN** no valid configured inventory path exists and `inventory/` exists in the project directory
- **THEN** the system uses the project `inventory/` directory

#### Scenario: Ancestor Ansible inventory is used
- **WHEN** no project inventory exists and an ancestor contains `ansible/inventory`
- **THEN** the system uses that ancestor inventory directory

#### Scenario: Fallback path is returned
- **WHEN** no inventory directory exists
- **THEN** the system returns the configured path if present, otherwise the project `inventory/` path
