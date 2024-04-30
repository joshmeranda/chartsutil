# Rebasing

Changing the base for an existing chart can be quite exhausting. The steps outlined in the [rancher/charts docs](https://github.com/rancher/charts/blob/dev-v2.9/docs/developing.md#rebasing-an-existing-package) while useable are less than ergonomic for larger charts like rancher-monitoring.

Eventually [a script](https://github.com/rancher/charts/pull/1936) came about that helped automate this process nicely; however, the error handling is not great, only fully supports git based branches, and bash is difficult to read / write processes like this with some small amount of moving parts. Additionally, we had to include the script in a throw-away commit that we had to remember to delete later (a monumental task for people like me).

## How It Works

At a high level, the rebase is esentially just preparing the target package, pulling in the charts from upstream, handling any conflicts, generating the patch, and updating the `package.yaml`.

When the user requests for a rebase, we immediately switch to a `quarantine` branch to protect the working development branch. From here we make a few pre-flight checks, prepare the package, and commit the changes to the chart working directory.

Next, on a new `charts-staging` branch, we pull and commit the upstream chart version. On the `quarantine` branch we pull those changes and allow the configeured resolver to handle any conflicting changes between those in a pacakges `generated-changes` and those found from the new upstream. Typically this will be done via an interactive shell allowing users to view and manually resolve those merge conflicts themselves, though *more* options may be exposed in the future. 

Once the package's working chart has been synced up to the desired upstream on the quarantine branch, we generate the patch, and update the `package.yaml` to reflect the new upstream.

Finally, we cherry-pick the `generated-changes` and `package.yaml` commits back to the main branch. Now we are done!

## Features

### Incremental vs Non-Incremental

For git based pacakges the rebase supports an incremental approach by calculating a list of commits between the current upstream and the target to handle each commit independantly. This can be done by providing the `--incremental` flag.

While allowed for non-git packages, it is not particulalry meaningful and the workflow would be identical for incremental and non-incremental rebases.

### Backups

When the `--backup` flag is present, we backup the updated prepared package to `.rebase-backup` something goes wrong later we don't lose all of our good progress. Especially nice for incremental rebases.