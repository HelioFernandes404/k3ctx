## MODIFIED Requirements

### Requirement: ArgoCD local port is shown after connect
The system SHALL print the ArgoCD local URL to stdout after a successful connect when the service was discovered and a tunnel was opened. When the login was not fully successful (not skipped but failed), the system SHALL print the `ArgocdLoginResult.Message` on the following line as a login hint.

#### Scenario: ArgoCD URL is printed on successful discovery
- **WHEN** `k3ctx connect` succeeds and an ArgoCD tunnel was opened
- **THEN** the system prints `ArgoCD: http://127.0.0.1:<port>` to stdout

#### Scenario: Login failure message is printed after ArgoCD URL
- **WHEN** `k3ctx connect` opens an ArgoCD tunnel but `argocd login` fails or argocd is not in PATH
- **THEN** the system prints a hint message on the line after the ArgoCD URL explaining the failure and how to login manually

## ADDED Requirements

### Requirement: ArgoCD login uses plaintext for HTTP ports
The system SHALL prefer `--plaintext` over `--insecure` when the discovered ArgoCD service port has name `http` or port number 80, to avoid TLS negotiation failures against plain-HTTP servers.

#### Scenario: HTTP-named port triggers plaintext login
- **WHEN** the discovered ArgoCD NodePort has name `http`
- **THEN** the system runs `argocd login --plaintext` instead of `argocd login --insecure`

#### Scenario: Port 80 triggers plaintext login
- **WHEN** the discovered ArgoCD NodePort has port number 80
- **THEN** the system runs `argocd login --plaintext` instead of `argocd login --insecure`

### Requirement: ArgoCD login retries with plaintext on insecure failure
The system SHALL retry `argocd login` with `--plaintext` when `argocd login --insecure` returns an error, to handle plain-HTTP ArgoCD servers without requiring inventory configuration.

#### Scenario: Insecure login failure triggers plaintext retry
- **WHEN** `argocd login --insecure` returns an error
- **THEN** the system retries once with `argocd login --plaintext` before reporting failure

#### Scenario: Plaintext retry success reports overall success
- **WHEN** `argocd login --insecure` fails and `argocd login --plaintext` succeeds
- **THEN** the ArgoCD login result is `Success: true`

#### Scenario: Both login attempts fail reports failure message
- **WHEN** both `argocd login --insecure` and `argocd login --plaintext` return errors
- **THEN** the system returns a non-success result with a message describing the failure
