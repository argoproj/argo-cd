-- CRD spec: https://gitops-promoter.readthedocs.io/en/latest/crd-specs/#restoreactivecommit
-- The action is hidden until the rollback has recorded status.activeSha.

local actions = {}
if obj.status ~= nil and obj.status.activeSha ~= nil and obj.status.activeSha ~= "" then
    actions["release-environment-block"] = {
        ["iconClass"] = "fa fa-fw fa-play",
        ["displayName"] = "Release Environment Block"
    }
end
return actions
