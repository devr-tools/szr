# Releasing szr

`szr` publishes GitHub releases from pushed tags that match `v*`.

## First release

```bash
git tag v0.1.0
git push origin main
git push origin v0.1.0
```

That tag push triggers `.github/workflows/release.yml`, which:

- builds `szr` for macOS and Linux on amd64 and arm64
- attaches `tar.gz` artifacts to the GitHub release
- publishes a `checksums.txt` file for those artifacts

## Future releases

```bash
git tag v0.1.1
git push origin main
git push origin v0.1.1
```

Use semantic versions so Homebrew and GitHub releases stay aligned.
