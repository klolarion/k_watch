# Security Policy

## Supported versions

Only the latest release receives security fixes. Upgrade to it before reporting an issue.

## Reporting a vulnerability

Please do not open a public issue for a security problem.

Report it privately through **Report a vulnerability** on the [Security tab](https://github.com/klolarion/k_watch/security) of this repository. If that is not available, email dev.kim.ops+kwatch@gmail.com.

Include the K-Watch version, how it is installed and exposed, and the steps that reproduce the problem.

## Scope

K-Watch has no login of its own. Anyone who can reach it can read what its ServiceAccount can read. This is documented behavior, not a vulnerability; see [Permissions and access control](README.md#permissions-and-access-control).

Examples of issues worth reporting:

- K-Watch creating, changing, or deleting cluster resources
- Reading data outside the chart's [read-only ClusterRole](charts/k-watch/templates/rbac.yaml), such as Secrets or ConfigMaps
- Reading files from the container through the HTTP server
- Script injection in the dashboard through resource names, labels, or events
