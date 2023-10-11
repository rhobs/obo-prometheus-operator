# RHOBS - Prometheus Operator Fork

This repository hosts a fork of the upstream Prometheus Operator with the API
Group changed from `monitoring.coreos.com` to `monitoring.rhobs`. This fork is
maintained specifically for the purpose of the [Observability
Operator](https://github.com/rhobs/observability-operator) which ships
Prometheus Operator as well. Since the targeted platform (OpenShift) already
has a Prometheus Operator, installing updated CRDs from newer version can
potentially break the platform. Hence this fork was created as workaround for
shipping newer version of Prometheus Operator without impacting on installed on
platform.

Check the
[rhobs/README.md](https://github.com/rhobs/obo-prometheus-operator/tree/rhobs-scripts/rhobs/README.md)
file (`rhobs-scripts` branch) for instructions about synchronizing this
downstream repository with upstream.
