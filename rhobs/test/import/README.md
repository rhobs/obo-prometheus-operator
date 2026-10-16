
This test checks if the packages exported by rhobs/obo-prometheus-operator are self sufficient, and do not pull any upstream dependency.

During the release process, the `go.mod` in this test is updated by `rhobs/make-release-commit.sh` script to the released version. The test is configured to be run by CI in `.github/workflows/rhobs-release.yaml`.
