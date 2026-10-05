# Brokoli ADBC Drivers

This repository publishes the curated native ADBC driver catalog used by Brokoli.

`index.json` is the stable catalog endpoint. Each entry names an immutable release archive for one operating-system and CPU-architecture pair and includes its SHA-256 digest. Brokoli verifies that digest before extracting or loading the driver.

## Publishing a driver

1. Build an archive containing `manifest.json` and the shared library.
2. Verify the archive and library checksums.
3. Upload the archive to an immutable GitHub release.
4. Add one entry per supported platform to `index.json`, using the release asset URL and archive SHA-256.
5. Open a pull request. The catalog workflow validates the index shape and digests before it can merge.

The catalog intentionally contains no packages until their provenance, license, and release artifact have been reviewed.
