# Chartsutil

This project is designed to augment the functionality of the rancher [charts-build-script](https://github.com/rancher/charts-build-scripts) by adding functionality that aids in the development of charts, but are not aiding in the specific build process and therefore don't quite fit the scoping of that tool.

## Building

```
make build
```

## Installing

For most use cases you will want to use our [install script](hack/install.sh) which should be able to detect the version of charts-build-scripts needed and install a version of chartsutil that supports that version:

```
curl https://raw.githubusercontent.com/joshmeranda/chartsutil/main/hack/install.sh | bash
```

## Example

You can try this tool for yourself by cloning [chartsutil-example](https://github.com/joshmeranda/chartsutil-example.git) and running the command below:

```
PACKAGE=chartsutil-example chartsutil rebase --backup --commit 933d8b2975efa50cda4dca6234e5e522b8f58cdc
```