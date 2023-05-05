# Wrapper Script for SBoM generation

- Find the [install.sh](../install.sh) script in the parent repo.
- The [Dockerfile](./Dockerfile) installs [dotnet-runtime-6.0_libs](https://github.com/canonical/chisel-releases/blob/ubuntu-22.04/slices/dotnet-runtime-6.0.yaml).
- Running the following command using [syft](https://github.com/anchore/syft) on the image generates the [sbom.json](./sbom.json):
```
syft packages <image> -o spdx-json
```
