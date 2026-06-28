# Renovate shared presets

Presets makes rules easier to maintain and reusable across multiple repositories.

# How to use a preset

Reference the preset in the `extends` field of the `renovate.json` file in the repository.  
Presets can reference other presets. (read more about [shared presets](https://docs.renovatebot.com/config-presets/))  

```json
{
  "extends": [
    "github>argoproj/argo-cd//renovate-presets/custom-managers/bash.json5"
]
}
```

### Note :

It would make sense to move this folder to a new repository in the future.  

Benefits:  
- Avoids consuming the repository's CI/CD resources.  
- Faster feedback loop for configuration changes.
- Avoid polluting the master git history.  
- The `renovate.json` in each repository can be simplified to only include a single presets :
	```json
	{
	  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
	  "extends": [
	    "github>argoproj/renovate-presets//argoproj/argo-cd/renovate.json5"
	  ],
	  // rules are empty and this file won't need to be modified again.
	  "packageRules": []
	}
	```
Inconvenient:  
- Owners of a repository can impact the configuration of all repositories. Use codeowners to reduce the risk.  

Example of repo structure :  
```shell
.
├── README.md
├── .github/CODEOWNERS
├── common.json5       # common presets for all repositories
├── fix/
│   └── openssf-merge-confidence-columns.json5
├── custom-managers/
│   ├── bash.json5
│   └── yaml.json5
└── argoproj/ # organization
    ├── argo-cd/ # repository
    │   ├── devtools.json5 # rules specific to the devtool (CI and dev environment...)
    │   ├── docs.json5 # rules specific to the docs folder.
    │   ├── # etc...
    │   └── renovate.json5 # this is the single preset referenced from the repository argopro/argo-cd.
    └── argo-rollouts/ # repository
        └── renovate.json5

```
