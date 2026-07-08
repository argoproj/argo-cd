module.exports = {
    platform: 'github',
    autodiscover: false,
    allowPostUpgradeCommandTemplating: true,
    allowedPostUpgradeCommands: [
        "make mockgen",
        "hack/installers/checksums/add-helm-checksums.sh",
        "hack/installers/checksums/add-kustomize-checksums.sh",
        "hack/installers/checksums/add-git-lfs-checksums.sh",
    ],
    binarySource: 'install',
    extends: [
        "github>argoproj/argo-cd//renovate-presets/commons.json5",
        "github>argoproj/argo-cd//renovate-presets/custom-managers/shell.json5",
        "github>argoproj/argo-cd//renovate-presets/custom-managers/yaml.json5",
        "github>argoproj/argo-cd//renovate-presets/fix/disable-all-updates.json5",
        "github>argoproj/argo-cd//renovate-presets/devtool.json5",
        "github>argoproj/argo-cd//renovate-presets/production-binaries.json5",
        "github>argoproj/argo-cd//renovate-presets/docs.json5",
        "group:aws-sdk-go-v2Monorepo",
        "github>argoproj/argo-cd//renovate-presets/fix/ignore-paths.json5"
    ],
    ignoreDeps: [
        'github.com/argoproj/argo-cd/gitops-engine/v3'
    ]
}