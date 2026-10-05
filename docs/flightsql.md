# Apache Flight SQL

Apache Flight SQL transports SQL results over Apache Arrow Flight RPC. This Brokoli package provides the native ADBC Flight SQL driver used by isolated `source_db` execution.

## Use in Brokoli

1. Install a supported Flight SQL driver release from **Drivers**.
2. Create a **Flight SQL** connection and select the installed driver identity.
3. Use that connection from a `source_db` node. The worker executes it in an isolated native child and writes Arrow IPC output by reference.

## Compatibility

The package is platform-specific. A pipeline needs a worker whose installed driver identity matches the connection exactly.

## Security

Brokoli verifies the catalog-pinned archive SHA-256 before installation and verifies the library SHA-256 before execution. Review the release lifecycle and published advisories before upgrading or installing.
