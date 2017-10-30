# Contributing to the Amazon SSM Agent

Contributions to the Amazon SSM Agent should be made via GitHub [pull
requests](https://github.com/aws/amazon-ssm-agent/pulls) and discussed using
GitHub [issues](https://github.com/aws/amazon-ssm-agent/issues).

### Before you start

If you would like to make a significant change, it's a good idea to first open
an issue to discuss it.

### Making the request

Development takes place against the `master` branch of this repository and pull
requests should be opened against that branch.

### Testing

Any contributions should pass all tests, including those not run by our
current CI system.

To execute all the tests simply run with `make quick-test`.

Alternatively you can run specific tests by specifying a package, sample below
`go test -v -gcflags "-N -l" github.com/aws/amazon-ssm-agent/agent/fileutil/...`

To execute all the integration tests simply run with `make quick-integtest`.

Alternatively you can run specific integration tests by specifying a package, sample below
`go test -v -gcflags "-N -l" -tags=integration github.com/aws/amazon-ssm-agent/agent/fileutil/...`

### Licensing

The Amazon SSM Agent is licensed under the Apache 2.0 License.
