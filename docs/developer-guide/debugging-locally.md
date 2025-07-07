# Debugging a local Argo CD instance

## Prerequisites
1. [Development Environment](development-environment.md)   
2. [Toolchain Guide](toolchain-guide.md)
3. [Development Cycle](development-cycle.md)
4. [Running Locally](running-locally.md)

## Preface
Please make sure you are familiar with running Argo CD locally using the [local toolchain](running-locally.md#start-local-services-local-toolchain).

When running Argo CD locally for manual tests, the quickest way to do so is to run all the Argo CD components together, as described in [Running Locally](running-locally.md), 

However, when you need to debug a single Argo CD component (for example, api-server, repo-server, etc), you will need to run this component separately in your IDE, using your IDE launch and debug configuration, while the other components will be running as described previously, using the local toolchain.

For the next steps, we will use Argo CD `api-server` as an example of running a component in an IDE.

## Configure your IDE

### Locate your component configuration in `Procfile`
The `Profile` is used by Goreman when running Argo CD locally with the local toolchain. It has all the needed component run configuration, and you will need to copy parts of this configuration to your IDE.

Example for `api-server` configuration in `Procfile`:
``` text
api-server: [ "$BIN_MODE" = 'true' ] && COMMAND=./dist/argocd || COMMAND='go run ./cmd/main.go' && sh -c "GOCOVERDIR=${ARGOCD_COVERAGE_DIR:-/tmp/coverage/api-server} FORCE_LOG_COLORS=1 ARGOCD_FAKE_IN_CLUSTER=true ARGOCD_TLS_DATA_PATH=${ARGOCD_TLS_DATA_PATH:-/tmp/argocd-local/tls} ARGOCD_SSH_DATA_PATH=${ARGOCD_SSH_DATA_PATH:-/tmp/argocd-local/ssh} ARGOCD_BINARY_NAME=argocd-server $COMMAND --loglevel debug --redis localhost:${ARGOCD_E2E_REDIS_PORT:-6379} --disable-auth=${ARGOCD_E2E_DISABLE_AUTH:-'true'} --insecure --dex-server http://localhost:${ARGOCD_E2E_DEX_PORT:-5556} --repo-server localhost:${ARGOCD_E2E_REPOSERVER_PORT:-8081} --port ${ARGOCD_E2E_APISERVER_PORT:-8080} --otlp-address=${ARGOCD_OTLP_ADDRESS} --application-namespaces=${ARGOCD_APPLICATION_NAMESPACES:-''} --hydrator-enabled=${ARGOCD_HYDRATOR_ENABLED:='false'}"
```
This configuration example will be used as the basis for the next steps.

### Configure component env variables
The component that you will run in your IDE for debugging (`api-server` in our case) will need env variables. Copy the env variables from `Procfile`, located in the `argo-cd` root folder of your development branch. The env variables are located before the `$COMMAND` section in the `sh -c` section of the component run command.
You can keep them in `.env` file and then have the IDE launch configuration point to that file. Obviously, you can adjust the env variables to your needs when debugging a specific configuration.

Example for an `api-server.env` file:
``` bash
ARGOCD_BINARY_NAME=argocd-server
ARGOCD_FAKE_IN_CLUSTER=true
ARGOCD_GNUPGHOME=/tmp/argocd-local/gpg/keys
ARGOCD_GPG_DATA_PATH=/tmp/argocd-local/gpg/source
ARGOCD_GPG_ENABLED=false
ARGOCD_LOG_FORMAT_ENABLE_FULL_TIMESTAMP=1
ARGOCD_SSH_DATA_PATH=/tmp/argocd-local/ssh
ARGOCD_TLS_DATA_PATH=/tmp/argocd-local/tls
ARGOCD_TRACING_ENABLED=1
FORCE_LOG_COLORS=1
KUBECONFIG=~/.kube/config
```

### Configure component IDE launch configuration
#### VSCode example
Next, you will need to create a launch configuration, with the relevant args. Copy the args from `Procfile`, located in the `argo-cd` root folder of your development branch. The args are located after the `$COMMAND` section in the `sh -c` section of the component run command.
Example for an `api-server` launch configuration, based on our above example for `api-server` configuration in `Procfile`: 
``` json
    {
      "name": "api-server",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "YOUR_CLONED_ARGO_CD_REPO_PATH/argo-cd/cmd",
      "args": [
        "--loglevel",
        "debug",
        "--redis",
        "localhost:6379",
        "--repo-server",
        "localhost:8081",
        "--dex-server",
        "http://localhost:5556",
        "--port",
        "8080",
        "--insecure"
      ],
      "envFile": "YOUR_ENV_FILES_PATH/api-server.env",
    }
```

#### Goland example
Next, you will need to create a launch configuration, with the relevant parameters. Copy the parameters from `Procfile`, located in the `argo-cd` root folder of your development branch. The parameters are located after the `$COMMAND` section in the `sh -c` section of the component run command.
Example for an `api-server` launch configuration, based on our above example for `api-server` configuration in `Procfile`: 
``` xml 
<component name="ProjectRunConfigurationManager">
  <configuration default="false" name="api-server" type="GoApplicationRunConfiguration" factoryName="Go Application">
    <module name="argo-cd" />
    <working_directory value="$PROJECT_DIR$" />
    <parameters value="--loglevel debug --redis localhost:6379 --insecure --dex-server http://localhost:5556 --repo-server localhost:8081 --port 8080  " />
    <EXTENSION ID="net.ashald.envfile">
      <option name="IS_ENABLED" value="true" />
      <option name="IS_SUBST" value="false" />
      <option name="IS_PATH_MACRO_SUPPORTED" value="false" />
      <option name="IS_IGNORE_MISSING_FILES" value="false" />
      <option name="IS_ENABLE_EXPERIMENTAL_INTEGRATIONS" value="false" />
      <ENTRIES>
        <ENTRY IS_ENABLED="true" PARSER="runconfig" IS_EXECUTABLE="false" />
        <ENTRY IS_ENABLED="true" PARSER="env" IS_EXECUTABLE="false" PATH="YOUR_ENV_FILES_PATH/api-server.env" />
      </ENTRIES>
    </EXTENSION>
    <kind value="DIRECTORY" />
    <package value="github.com/argoproj/argo-cd/v" />
    <directory value="$PROJECT_DIR$/cmd" />
    <filePath value="$PROJECT_DIR$" />
    <method v="2" />
  </configuration>
</component>
```

## Run Argo CD without the debugged component
Next, we need to run all Argo CD components, except for the debugged component (cause we will run this component separately in the IDE).
There is a mix-and-match approach to running the other components - you can run them in your K8s cluster or locally with the local toolchain.
Below are the different options.

### Run the other components locally
#### Run with "make start-local"
`make start-local` runs all the components by default, but it is also possible to run it with a whitelist of components, enabling the separation we need.

So for the case of debugging the `api-server`, run:
`make start-local ARGOCD_START="notification applicationset-controller repo-server redis dex controller ui"` 

#### Run with "make run"
`make run` runs all the components by default, but it is also possible to run it with a blacklist of components, enabling the separation we need.

So for the case of debugging the `api-server`, run:
`make run exclude=api-server` 

### Run the other components in your K8s cluster
It is also possible to run the other components in your K8s cluster, by scaling out their relevant deployments/stateful sets replicas to 1.
In our example of debugging the `api-server`, all the other Argo CD deployments/stateful sets will need to be scaled up to 1 replica, and `argocd-server` will remain with 0 replicas.

## Run Argo CD debugged component from your IDE
Finally, run the component you wish to debug from your IDE and make sure it does not have any errors.

## Important
In any of the above methods for running the other Argo CD components separately, you need to make sure they don't step on each other's feet - meaning that each component needs to be up exactly once, be it up in the cluster, run locally with the local toolchain or run from your IDE. Otherwise you may be getting errors about ports not available or even debugging a process that does not run your changed code. 