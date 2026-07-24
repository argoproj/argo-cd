# GitHub Actions

GitHub is an OAuth identity provider which can be used in GitHub Actions
to generate tokens that identifies the repository and where it runs.

See: <https://docs.github.com/en/actions/security-for-github-actions/security-hardening-your-deployments/about-security-hardening-with-openid-connect>

You need to use OAuth 2.0 Token Exchange. Some identity providers supports this
out of the box such as Dex. Alternatively, you can federate the GitHub OIDC
token directly into Microsoft Entra ID and have Argo CD trust Entra via
`oidc.config`.

## Using Dex

Edit the `argocd-cm` and configure the `dex.config` section:

```yaml
dex.config: |
  connectors:
    - type: oidc
      id: github-actions
      name: GitHub Actions
      config:
        issuer: https://token.actions.githubusercontent.com/
        # If using GitHub Enterprise Server, then use this issuer:
        #issuer: https://github.example.com/_services/token
        scopes: [openid]
        userNameKey: sub
        insecureSkipEmailVerified: true
```

ArgoCD automatically generates a static client named `argo-cd-cli` that you can use to get your token from a GitHub Action.

Here is an example of GitHub Action that will retrieve a valid Argo CD authentication token from Dex and use it to perform action with the CLI:

```yaml
name: argocd-test

on:
  pull_request:

permissions:
  id-token: write # This is required for requesting the JWT

jobs:
  argocd-test:
    runs-on:
      group: ephemeral_runners
    steps:
      # Actions have access to two special environment variables ACTIONS_CACHE_URL and ACTIONS_RUNTIME_TOKEN.
      # Inline step scripts in workflows do not see these variables.
      - uses: actions/github-script@v6
        id: script
        timeout-minutes: 10
        with:
          debug: true
          script: |
            const token = process.env['ACTIONS_RUNTIME_TOKEN']
            const runtimeUrl = process.env['ACTIONS_ID_TOKEN_REQUEST_URL']
            core.setOutput('TOKEN', token.trim())
            core.setOutput('IDTOKENURL', runtimeUrl.trim())

      - name: Obtain access token
        id: idtoken
        run: |
          # get an token from github
          echo "getting token from GitHub"
          GH_TOKEN_RESPONSE=$(curl -sSf \
            "${{steps.script.outputs.IDTOKENURL}}" \
            -H "Authorization: bearer  ${{steps.script.outputs.TOKEN}}" \
            -H "Accept: application/json; api-version=2.0" \
            -H "Content-Type: application/json" \
            -d "{}" \
          )
          GH_TOKEN=$(jq -r .value <<< $GH_TOKEN_RESPONSE)
          echo "::add-mask::$GH_TOKEN"

          # exchange it for a dex token
          DEX_URL="https://argocd.example.com/api/dex/token"
          echo "getting access token from Dex: $DEX_URL"
          DEX_TOKEN_RESPONSE=$(curl -sSf \
              "$DEX_URL" \
              --user argo-cd-cli: \
              --data-urlencode "connector_id=github-actions" \
              --data-urlencode "grant_type=urn:ietf:params:oauth:grant-type:token-exchange" \
              --data-urlencode "scope=openid email profile federated:id" \
              --data-urlencode "requested_token_type=urn:ietf:params:oauth:token-type:access_token" \
              --data-urlencode "subject_token=$GH_TOKEN" \
              --data-urlencode "subject_token_type=urn:ietf:params:oauth:token-type:id_token")
          DEX_TOKEN=$(jq -r .access_token <<< $DEX_TOKEN_RESPONSE)

          if [[ -z "$DEX_TOKEN" ]]; then
            echo "::error::No token found in dex response"
            exit 1
          fi

          echo "::add-mask::$(echo "$DEX_TOKEN" | base64 -w0)"
          echo "::add-mask::$DEX_TOKEN"
          echo "dex-token=$DEX_TOKEN" >> "$GITHUB_OUTPUT"
          # use $DEX_TOKEN

      - name: Setup ArgoCD CLI
        run: |
          curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64
          mkdir -p "$RUNNER_TEMP/argocd"
          install -m 555 argocd-linux-amd64 "$RUNNER_TEMP/argocd/argocd"
          rm argocd-linux-amd64
          echo "$RUNNER_TEMP/argocd" >> "$GITHUB_PATH"

      - name: Use CLI in some commands
        env:
          ARGOCD_AUTH_TOKEN: ${{ steps.idtoken.outputs.dex-token }}
          ARGOCD_SERVER: argocd.example.com
          ARGOCD_OPTS: --grpc-web
        run: |
          set -x
          argocd version
          argocd account get-user-info
          argocd proj list
          argocd app list
```

## Using Microsoft Entra ID

Instead of exchanging the GitHub OIDC token through Dex, you can federate it
directly into Microsoft Entra ID using
[Workload Identity Federation](https://learn.microsoft.com/en-us/entra/workload-id/workload-identity-federation).
Entra ID trusts the GitHub-issued JWT as a client assertion and returns an
access token for the Argo CD App Registration — no client secret is stored in
GitHub.

This assumes Argo CD is already configured to trust Entra ID directly via
`oidc.config` as described in
[Microsoft Entra ID App Registration Auth using OIDC](microsoft.md#entra-id-app-registration-auth-using-oidc).

### Prerequisites

1. An Entra ID App Registration for Argo CD (the one referenced by
   `oidc.config`).
2. An App Registration representing the GitHub Actions workload, with a
   **Federated credential** configured:
    - **Scenario:** GitHub Actions deploying Azure resources
    - **Organization / Repository:** your GitHub org and repo
    - **Entity type:** Pull request (or Branch / Environment as appropriate)
    - Keep the default audience `api://AzureADTokenExchange`.
3. The workload App Registration must be authorized to request tokens for the
   Argo CD App Registration (the `{ARGOCD_ENTRA_APP_ID}/.default` scope).

### Configuring Argo CD

Edit the `argocd-cm` and configure the `oidc.config` section so Argo CD trusts
tokens issued by your Entra tenant:

```yaml
oidc.config: |
  name: GitHub Actions
  issuer: https://sts.windows.net/{tenant-id}/
  clientID: {argocd-app-registration-client-id}
  allowedAudiences:
    - {argocd-app-registration-client-id}
```

> [!NOTE]
> Tokens minted via the `client_credentials` grant are v1.0 access tokens
> issued by `https://sts.windows.net/{tenant-id}/`. Make sure the `issuer` in
> `oidc.config` matches (not the `login.microsoftonline.com/{tenant}/v2.0`
> form), and that the token's `aud` claim — the Argo CD App Registration client
> ID — is listed in `allowedAudiences`.

Here is an example of a GitHub Action that retrieves a valid Argo CD
authentication token by federating the GitHub OIDC JWT into Entra ID, and uses
it to perform actions with the CLI:

```yaml
name: argocd-test

on:
  pull_request:

permissions:
  id-token: write # This is required for requesting the JWT

jobs:
  argocd-test:
    runs-on: ubuntu-latest
    env:
      ARGOCD_SERVER: argocd.example.com
      ARGOCD_OPTS: --grpc-web
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0        # need the base ref to diff against

      - name: Setup ArgoCD CLI
        run: |
          curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64
          mkdir -p "$RUNNER_TEMP/argocd"
          install -m 555 argocd-linux-amd64 "$RUNNER_TEMP/argocd/argocd"
          echo "$RUNNER_TEMP/argocd" >> "$GITHUB_PATH"
          rm argocd-linux-amd64

      # Which ApplicationSets changed in this PR? Empty output => nothing to do.
      - name: Select changed ApplicationSets
        id: select
        run: |
          set -euo pipefail
          git fetch --no-tags --depth=1 origin "${{ github.base_ref }}"
          targets="$(git diff --name-only "origin/${{ github.base_ref }}...HEAD" \
            -- 'appSet/**/*.yaml' 'appSet/**/*.yml' 'appSet/*.yaml' 'appSet/*.yml' \
            | tr '\n' ' ' | sed 's/ *$//')"
          echo "Changed appsets: ${targets:-<none>}"
          echo "targets=${targets}" >> "$GITHUB_OUTPUT"

      - name: Authenticate (GitHub OIDC -> Entra) and validate
        if: steps.select.outputs.targets != ''
        env:
          # Prefer repo/org Actions variables over inline literals.
          AZURE_TENANT_ID: ${{ vars.AZURE_TENANT_ID }}
          AZURE_CLIENT_ID: ${{ vars.AZURE_CLIENT_ID }}
          ARGOCD_ENTRA_APP_ID: ${{ vars.ARGOCD_ENTRA_APP_ID }}
        run: |
          set -euo pipefail
          if [[ -z "${ARGOCD_SERVER:-}" ]]; then
            echo "::error::ARGOCD_SERVER is empty. Define the repo/org Actions variable ARGOCD_SERVER (host only)."
            exit 1
          fi

          # 1) GitHub OIDC JWT, audience fixed to Entra's WIF exchange audience.
          gh_jwt="$(curl -sSf \
            "${ACTIONS_ID_TOKEN_REQUEST_URL}&audience=api://AzureADTokenExchange" \
            -H "Authorization: bearer ${ACTIONS_ID_TOKEN_REQUEST_TOKEN}" \
            -H "Accept: application/json; api-version=2.0" \
            | jq -r '.value')"

          # 2) Federate into Entra: the GitHub JWT is the client assertion, and we
          #    request a token whose audience is Argo CD's Entra app (no secret).
          resp="$(curl -sSf \
            "https://login.microsoftonline.com/${AZURE_TENANT_ID}/oauth2/v2.0/token" \
            --data-urlencode "grant_type=client_credentials" \
            --data-urlencode "client_id=${AZURE_CLIENT_ID}" \
            --data-urlencode "client_assertion_type=urn:ietf:params:oauth:client-assertion-type:jwt-bearer" \
            --data-urlencode "client_assertion=${gh_jwt}" \
            --data-urlencode "scope=${ARGOCD_ENTRA_APP_ID}/.default")"

          argocd_token="$(jq -r '.access_token // empty' <<< "$resp")"
          if [[ -z "$argocd_token" ]]; then
            echo "::error::Entra token exchange failed: $(jq -rc '{error, error_description}' <<< "$resp" 2>/dev/null || echo "$resp")"
            exit 1
          fi
          echo "::add-mask::${argocd_token}"

          # 3) Argo CD trusts Entra directly via oidc.config; pass the token as-is.
          export ARGOCD_AUTH_TOKEN="${argocd_token}"
          argocd version
          argocd account get-user-info
          argocd app list
```

> [!NOTE]
> The GitHub OIDC token must be requested with the audience
> `api://AzureADTokenExchange`. This is the fixed audience Entra ID expects for
> workload identity federation token exchanges.

## Configuring RBAC

When using ArgoCD v3.0.0 or later, then you define your `policy.csv` like so:

```yaml
configs:
  rbac:
    policy.csv: |
      p, repo:my-org/my-repo:pull_request, projects, get, my-project, allow
      p, repo:my-org/my-repo:pull_request, applications, get, my-project/*, allow
      p, repo:my-org/my-repo:pull_request, applicationsets, get, my-project/*, allow
```

When authenticating through **Microsoft Entra ID**, the subject differs. Tokens
obtained via the `client_credentials` grant identify the workload App
Registration itself, not a repository, so the subject is the **object ID of the
service principal** (`sub`/`oid` claim) rather than a `repo:org/repo:ref`
subject. Bind your role to that ID instead:

```yaml
configs:
  rbac:
    policy.csv: |
      p, role:ci-argo-app, applications, get, */*, allow
      g, {obj-id}, role:ci-argo-app
```

Where `{obj-id}` is the object ID of the service
principal used by the workflow. Grant only the minimum permissions the workflow
needs — in this example, read-only access to applications.

More info: [RBAC Configuration](../rbac.md)

> [!NOTE]
> Defining policies are not supported on ArgoCD v2.
> To define policies, please [upgrade](../upgrading/overview.md)
> to v3.0.0 or later.