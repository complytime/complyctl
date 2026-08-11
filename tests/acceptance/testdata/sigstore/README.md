# TEST PKI — GENERATED AT RUNTIME

The key material for the `complyctl` verification acceptance test stack
(`tests/acceptance/`, `make test-acceptance-verify`) is generated on every
test run by the `generate-pki` Docker Compose init service into the
`pki-material` volume.

No key material is committed to this repository.

| File          | Purpose                                                           | Location     |
|---------------|-------------------------------------------------------------------|--------------|
| `root.pem`    | Fulcio `fileca` self-signed CA certificate (EC P-256, 10 yr)      | pki-material |
| `root.key`    | The CA private key (PEM, AES-256-CBC, passphrase `fulcio`)        | pki-material |
| `privkey.pem` | CTFE log signing key (PEM, legacy AES-256-CBC, passphrase `ctfe`) | pki-material |
| `ctfe.pub`    | CTFE log public key (used for `cosign trusted-root --ctfe-key`)   | pki-material |

These keys are ephemeral test fixtures with zero security value. They are
generated fresh each time the verification test suite runs.
