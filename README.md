# Brokoli ADBC Drivers

This repository publishes the curated native ADBC driver catalog used by Brokoli.

`index.json` is the stable catalog endpoint. Each entry names an immutable release archive for one operating-system and CPU-architecture pair and includes its SHA-256 digest. Brokoli verifies that digest before extracting or loading the driver.

## Publishing a driver

1. Build an archive containing `manifest.json` and the shared library.
2. Verify the archive and library checksums.
3. Upload the archive to an immutable GitHub release.
4. Add one entry per supported platform to `index.json`, using the release asset URL and archive SHA-256.
5. Open a pull request. The catalog workflow validates the index shape, then installs every Linux entry with Brokoli's own installer and loads it through the ADBC driver manager, in an environment shaped like Brokoli's native worker image (`tools/loadcheck`). An entry that does not download, verify, match its manifest, or load fails the pull request.

A driver must be self-contained: its library may depend only on the base C and C++ runtime (`libc`, `libm`, `libdl`, `libpthread`, `librt`, `libresolv`, `libstdc++`, `libgcc_s`). A database client library it needs, such as `libpq` or `libsqlite3`, must be statically linked or bundled, or the driver loads only where that library happens to be installed. The upstream Python wheels on PyPI are built this way; `package-registry-driver.yml` packages a driver from one, pinned by digest.

Run the check locally with:

```sh
docker build -t catalog-loadcheck tools/loadcheck
docker run --rm -v "$PWD/index.json:/catalog/index.json:ro" catalog-loadcheck
```

The catalog intentionally contains no packages until their provenance, license, and release artifact have been reviewed.
