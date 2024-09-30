This directory contains e2e-style UI tests.

To run the tests, first make sure you have an Argo CD instance available on http://localhost:4000. You can do this by
running `make start-e2e` or `make start-e2e-local` in the root of the Argo CD repository.

Then, run `yarn install` to install the necessary dependencies, and `yarn test` to run the tests.

By default, the tests run in headless mode. To run the tests in a browser window, set `IS_HEADLESS` to `false` in the
`src/.env` file.
