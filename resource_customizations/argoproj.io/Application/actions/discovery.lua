local actions = {}
local autoSyncEnabled = false
if obj.spec ~= nil and obj.spec.syncPolicy ~= nil and obj.spec.syncPolicy.automated ~= nil then
    local enabled = obj.spec.syncPolicy.automated.enabled
    autoSyncEnabled = (enabled == true or enabled == nil)
end
actions["toggle-auto-sync"] = {
    ["displayName"] = autoSyncEnabled and "Disable Auto-Sync" or "Enable Auto-Sync",
    ["disabled"] = false
}
return actions
