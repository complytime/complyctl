# Tests using Testing Farm

[Testing Farm](https://packit.dev/docs/configuration/upstream/tests) is Packit's testing system.
Test execution is managed by tmt tool.

The entry of the testing farm tests is located at [.packit.yaml](../.packit.yaml), in the job named `tests`.

The `tests` job requires `copr_build` job to be built before running tests,
so the built packages are automatically installed in the testing environment.

The [Testing Farm documentation](https://packit.dev/docs/configuration/upstream/tests) gives information on how to include or modify tests.
