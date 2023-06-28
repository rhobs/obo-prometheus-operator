# RHOBS - Prometheus Operator Fork


This repo hosts a fork of upstream Prometheus Operator with the API Group
changed from `monitoring.coreos.com` to `monitoring.rhobs`. This fork is
maintained specifically for the purpose of Observability Operator which ships
Prometheus Operator as well. Since the targeted platform - OpenShift already
has a Prometheus Operator, installing updated CRDs from newer version can
potentially break the platform. Hence this fork was created as workaround for
shipping newer version of Prometheus Operator without impacting on installed on
platform.
