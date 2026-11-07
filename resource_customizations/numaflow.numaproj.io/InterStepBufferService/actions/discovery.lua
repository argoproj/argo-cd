local actions = {}
actions["force-promote"] = {
  ["disabled"] = true,
  ["iconClass"] = "fa-solid fa-fw fa-forward"
}
actions["force-promote-chain"] = {
  ["disabled"] = true,
  ["displayName"] = "Force Promote Chain",
  ["iconClass"] = "fa-solid fa-fw fa-forward-fast"
}

-- force-promote / force-promote-chain
-- Numaplane ignores an individual force-promote while the trial chain has failed,
-- so only one of the two actions is enabled at a time
local labels = obj.metadata.labels or {}
local isTrial = labels["numaplane.numaproj.io/upgrade-state"] == "trial"
local chainFailed = labels["numaplane.numaproj.io/progressive-chain-failed"] == "true"
local forcePromoteRequested = labels["numaplane.numaproj.io/force-promote"] == "true"
local forcePromoteChainRequested = labels["numaplane.numaproj.io/force-promote-chain"] == "true"
if isTrial and not chainFailed and not forcePromoteRequested and not forcePromoteChainRequested then
  actions["force-promote"]["disabled"] = false
end
if isTrial and chainFailed and not forcePromoteChainRequested then
  actions["force-promote-chain"]["disabled"] = false
end

return actions
