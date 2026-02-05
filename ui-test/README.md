# Argo-cd UI Test

## Context and use

At the moment, the UI tests are not mature enough to be embedded in the global testing systems.
However, you can run them through a docker image against any ArgoCD instance.

## Initial Configuration

Prepare the file `.env` to locate your running ArgoCD instance.
You will need a running instance of ArgoCD available to run the UI tests.

The tests output, namely session screenshots will be persisted in the local `_logs` directory. 

```bash
cp .env.tpl .env
mkdir _logs
```

## Run

You can build and run the docker image, this will run all existing tests in [test_cases](./src/test_cases).
```bash
docker build . -t ui-test && docker run --rm --network=host -v ./_logs:/root/.npm/_logs ui-test
```

## Add New Test

Create a new `test_*.ts` file under the directory [test_cases](./src/test_cases), with a test method defining the following interface. 
```typescript
export async function doTest(navigation: Navigation) {
    // Test content ...
}
```

The method will be picked up and run as part of the test suite.
