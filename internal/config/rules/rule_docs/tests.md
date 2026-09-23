Review this file as a test: it is only worth having if it fails when the behaviour it names breaks.
- Assertions: each test must assert the behaviour its name and comments promise; flag tests with no real assertion, assertions that can never fail, and checks that only restate the mock or fixture.
- Fixtures and data: fixtures, expected values and comments must agree with each other and with the code under test; flag stale expectations and copy-pasted cases that test the same thing twice.
- Isolation: shared state, global environment, time, randomness, network or file-system access that can make the test order-dependent or flaky.
- Coverage of the change: an edge case or error path the production change introduces that no test exercises, when the test file claims to cover that code.
- Also apply the language's usual correctness checks to the test code itself.
Do not ask for more tests in general, and do not comment on naming or style.
