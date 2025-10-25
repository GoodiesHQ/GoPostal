# GoPostal
A simple agent to relay SMTP (plaintext, TLS, or STARTTLS) to the MS Graph API for sending mail. Authentication is optional. Ideal to use instead of public SMTP relays or for devices which lack modern authentication support for SMTP workflows.

## Configuration

```yml
recv:
  # The domain this receiver is responsible for (used in HELO/EHLO and other responses)
  # Leave empty to accept any identity name
  domain: "mail.example.local"

  # Listeners define the ports and protocols this receiver will accept connections on
  listeners:
    # Example plaintext unauthenticated SMTP server
    - name: "server-25"
      port: 25
      type: "smtp"          # smtp | smtps | starttls
      require_auth: false    # allow unauthenticated on this listener
    
    # Example authenticated implicit TLS server (requires certificate)
    - name: "server-465"
      port: 465
      type: "smtps"         # implicit TLS
      require_auth: true
      tls:
        cert_file: "/path/to/cert.pem"
        key_file:  "/path/to/key.pem"
    
    # Example authenticated explicit TLS server (requires certificates)
    - name: "server-587"
      port: 587
      type: "starttls"
      require_auth: true
      tls:
        cert_file: "/path/to/cert.pem"
        key_file:  "/path/to/key.pem"

  # Global source IP policy (remove or use `allowed_ips: []` to allow all source IPs)
  # Example: Allow all non-public IP addresses
  allowed_ips:
    - 10.0.0.0/8
    - 172.16.0.0/12
    - 192.168.0.0/16
    - 100.64.0.0/10
    - 127.0.0.0/8
    - ::1/128

  # Authentication capability
  auth:
    # mode: disabled | anonymous | plain | plain-any
    # - disabled: no AUTH advertised; allowed only if listener[].require_auth=false
    # - anonymous: AUTH ANONYMOUS advertised/accepted (rare; usually not recommended)
    # - plain: AUTH PLAIN against the provided list of users
    # - plain-any: AUTH PLAIN but accept any username/password (testing only)
    mode: "plain"
    # TODO: Change this from plaintext passwords to bcrypt password hashes
    credentials:
      - username: "alice"
        password: "Passw0rd1"
      - username: "bob"
        password: "Passw0rd2"

  # Sender policy - if both addresses and domains are empty, all sources are allowed
  valid_from:
    # specific allowed sender email addresses (remove or use `addresses: []` to allow all)
    addresses:
      - "user@example.com"
    # allow any sender from these domains (remove or use `domains: []` to allow all)
    domains:
      - "example.org"
  # Recipient policy - if both addresses and domains are empty, all destinations are allowed
  valid_to:
    # specific allowed recipient email addresses (remove or use `addresses: []` to allow all)
    addresses:
      - "a@example.com"
      - "b@example.com"
    # allow any recipient from these domains (remove or use `domains: []` to allow all)
    domains:
      - "example.org"

  # Operational limits and timeouts (defaults shown)
  limits:
    max_size:       26214400     # 25 MiB
    max_recipients: 100
    timeout:        "30s"

send:
  graph:
    # Azure App Registration information. Be sure to allow Application permission Mail.Send
    tenant_id: "0564701c-1f01-4120-af7f-a5f10192a73d"
    client_id: "a8a5c463-0ac6-4e28-8b4f-22a2551823c1"
    client_secret: "<GRAPH_CLIENT_SECRET>"
    # Optional submission identity for Graph (if empty, uses the `from` address provided by SMTP)
    mailbox: "notifications@example.com"
    timeout: "10s"
    retries: 3
    backoff: "5s"
    # If false, the server will not start unless the application can acquire a valid token from the MS Graph API
    allow_start_without_graph: false
```