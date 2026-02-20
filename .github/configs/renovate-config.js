module.exports = {
    platform: 'github',
    gitAuthor: 'renovate[bot] <renovate[bot]@users.noreply.github.com>',
    autodiscover: false,
    allowPostUpgradeCommandTemplating: true,
    allowedPostUpgradeCommands: ["make mockgen", "make codegen-local"],
    binarySource: 'install',
    extends: [
        "github>argoproj/argo-cd//renovate-presets/commons.json5",
        "github>argoproj/argo-cd//renovate-presets/custom-managers/shell.json5",
        "github>argoproj/argo-cd//renovate-presets/custom-managers/yaml.json5",
        "github>argoproj/argo-cd//renovate-presets/custom-managers/docker-pull.json5",
        "github>argoproj/argo-cd//renovate-presets/fix/disable-all-updates.json5",
        "github>argoproj/argo-cd//renovate-presets/devtool.json5",
        "github>argoproj/argo-cd//renovate-presets/docs.json5"
    ]
}