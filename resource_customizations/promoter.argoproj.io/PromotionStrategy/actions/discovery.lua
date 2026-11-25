-- CRD spec: https://gitops-promoter.readthedocs.io/en/latest/crd-specs/#promotionstrategy
-- Rolls an environment back by creating a RestoreActiveCommit. promotionStrategyRef, branch, and sha are immutable.

local actions = {}
actions["rollback-environment"] = {
    ["iconClass"] = "fa fa-fw fa-undo",
    ["displayName"] = "Rollback Environment",
    ["params"] = {
        {["name"] = "branch"},
        {["name"] = "sha"}
    }
}
return actions
