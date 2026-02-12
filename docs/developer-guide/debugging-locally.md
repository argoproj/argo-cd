# Debugging a local Argo CD instance

## Prerequisites
1. [Development Environment](development-environment.md)   
2. [Toolchain Guide](toolchain-guide.md)
3. [Development Cycle](development-cycle.md)
4. [Running Locally](running-locally.md)

## Preface
Please make sure you are familiar with running Argo CD locally using the [local toolchain](running-locally.md#start-local-services-local-toolchain).

When running Argo CD locally for manual tests, the quickest way to do so is to run all the Argo CD components together, as described in [Running Locally](running-locally.md), 

However, when you need to debug a single Argo CD component (for example, `api-server`, `repo-server`, etc), you will need to run this component separately in your IDE, using your IDE launch and debug configuration, while the other components will be running as described previously, using the local toolchain.

For the next steps, we will use Argo CD `api-server` as an example of running a component in an IDE.

## Configure your IDE

### Locate your component configuration in `Procfile`
The `Procfile` is used by Goreman when running Argo CD locally with the local toolchain. The [latest Procfile](https://github.com/argoproj/argo-cd/blob/master/Procfile) is located in the top-level directory in your cloned Argo CD repo folder. It contains all the needed component run configuration, and you will need to copy parts of this configuration to your IDE. 

Example for `api-server` configuration in `Procfile`:
``` text
api-server: [ "$BIN_MODE" = 'true' ] && COMMAND=./dist/argocd || COMMAND='go run ./cmd/main.go' && sh -c "GOCOVERDIR=${ARGOCD_COVERAGE_DIR:-/tmp/coverage/api-server} FORCE_LOG_COLORS=1 ARGOCD_FAKE_IN_CLUSTER=true ARGOCD_TLS_DATA_PATH=${ARGOCD_TLS_DATA_PATH:-/tmp/argocd-local/tls} ARGOCD_SSH_DATA_PATH=${ARGOCD_SSH_DATA_PATH:-/tmp/argocd-local/ssh} ARGOCD_BINARY_NAME=argocd-server $COMMAND --loglevel debug --redis localhost:${ARGOCD_E2E_REDIS_PORT:-6379} --disable-auth=${ARGOCD_E2E_DISABLE_AUTH:-'true'} --insecure --dex-server http://localhost:${ARGOCD_E2E_DEX_PORT:-5556} --repo-server localhost:${ARGOCD_E2E_REPOSERVER_PORT:-8081} --port ${ARGOCD_E2E_APISERVER_PORT:-8080} --otlp-address=${ARGOCD_OTLP_ADDRESS} --application-namespaces=${ARGOCD_APPLICATION_NAMESPACES:-''} --hydrator-enabled=${ARGOCD_HYDRATOR_ENABLED:='false'}"
```
This configuration example will be used as the basis for the next steps.

> [!NOTE]
> The Procfile for a component may change with time. Please go through the Procfile and make sure you use the latest configuration for debugging.

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
KUBECONFIG=/Users/<YOUR_USERNAME>/.kube/config # Must be an absolute full path
... 
# and so on, for example: when you test the app-in-any-namespace feature, 
# you'll need to add ARGOCD_APPLICATION_NAMESPACES to this list 
# only for testing this functionality and remove it afterwards.
```

### Install DotENV / EnvFile plugin
Using the market place / plugin manager of your IDE. The below example configurations require the plugin to be installed.


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
      "envFile": "YOUR_ENV_FILES_PATH/api-server.env", # Assuming you installed DotENV plugin
    }
```

#### Goland example
Next, you will need to create a launch configuration, with the relevant parameters. Copy the parameters from `Procfile`, located in the `argo-cd` root folder of your development branch. The parameters are located after the `$COMMAND` section in the `sh -c` section of the component run command.
Example for an `api-server` launch configuration snippet, based on our above example for `api-server` configuration in `Procfile`: 
``` xml 
<component name="ProjectRunConfigurationManager">
  <configuration default="false" name="api-server" type="GoApplicationRunConfiguration" factoryName="Go Application">
    <module name="argo-cd" />
    <working_directory value="$PROJECT_DIR$" />
    <parameters value="--loglevel debug --redis localhost:6379 --insecure --dex-server http://localhost:5556 --repo-server localhost:8081 --port 8080" />
    <EXTENSION ID="net.ashald.envfile"> <!-- Assuming you installed the EnvFile plugin-->
      <option name="IS_ENABLED" value="true" />
      <option name="IS_SUBST" value="false" />
      <option name="IS_PATH_MACRO_SUPPORTED" value="false" />
      <option name="IS_IGNORE_MISSING_FILES" value="false" />
      <option name="IS_ENABLE_EXPERIMENTAL_INTEGRATIONS" value="false" />
      <ENTRIES>
        <ENTRY IS_ENABLED="true" PARSER="runconfig" IS_EXECUTABLE="false" />
        <ENTRY IS_ENABLED="true" PARSER="env" IS_EXECUTABLE="false" PATH="<YOUR_ENV_FILES_PATH>/api-server.env" />
      </ENTRIES>
    </EXTENSION>
    <kind value="DIRECTORY" />
    <directory value="$PROJECT_DIR$/cmd" />
    <filePath value="$PROJECT_DIR$" />
    <method v="2" />
  </configuration>
</component>
```

> [!NOTE]
> As an alternative to importing the above file to Goland, you can create a Run/Debug Configuration using the official [Goland docs](https://www.jetbrains.com/help/go/go-build.html) and just copy the `parameters`, `directory` and `PATH` sections from the example above (specifying `Run kind` as `Directory` in the Run/Debug Configurations wizard)

## Run Argo CD without the debugged component
Next, we need to run all Argo CD components, except for the debugged component (cause we will run this component separately in the IDE).
There is a mix-and-match approach to running the other components - you can run them in your K8s cluster or locally with the local toolchain.
Below are the different options.

### Run the other components locally
#### Run with "make start-local"
`make start-local` runs all the components by default, but it is also possible to run it with a whitelist of components, enabling the separation we need.

So for the case of debugging the `api-server`, run:
`make start-local ARGOCD_START="notification applicationset-controller repo-server redis dex controller ui"` 

> [!NOTE]
> By default, the api-server in this configuration runs with auth disabled. If you need to test argo cd auth-related functionality, run `export ARGOCD_E2E_DISABLE_AUTH='false' && make start-local`
#### Run with "make run"
`make run` runs all the components by default, but it is also possible to run it with a blacklist of components, enabling the separation we need.

So for the case of debugging the `api-server`, run:
`make run exclude=api-server` 

#### Run with "goreman start"
`goreman start` runs all the components by default, but it is also possible to run it with a whitelist of components, enabling separation as needed.

To debug the `api-server`, run:
`goreman start notification applicationset-controller repo-server redis dex controller ui` 

## Run Argo CD debugged component from your IDE
Finally, run the component you wish to debug from your IDE and make sure it does not have any errors.

## Important
When running Argo CD components separately, ensure components aren't creating conflicts - each component needs to be up exactly once, be it running locally with the local toolchain or running from your IDE. Otherwise you may get errors about ports not available or even debugging a process that does not contain your code changes. 
