local actions = {}
local autoSyncEnabled = false
if obj.spec ~= nil and obj.spec.syncPolicy ~= nil and obj.spec.syncPolicy.automated ~= nil then
    autoSyncEnabled = true
end
actions["toggle-auto-sync"] = {
    ["displayName"] = autoSyncEnabled and "Disable Auto-Sync" or "Enable Auto-Sync",
    ["disabled"] = false
}
return actions
