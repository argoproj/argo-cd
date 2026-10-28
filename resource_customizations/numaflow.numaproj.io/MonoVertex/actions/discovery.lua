local actions = {}
actions["pause"] = {
  ["disabled"] = true,
  ["iconClass"] = "fa-solid fa-fw fa-pause"
}
actions["unpause-gradual"] = {
  ["disabled"] = true,
  ["displayName"] = "Unpause (gradual)",
  ["iconClass"] = "fa-solid fa-fw fa-play"
}
actions["unpause-fast"] = {
  ["disabled"] = true,
  ["displayName"] = "Unpause (fast)",
  ["iconClass"] = "fa-solid fa-fw fa-play"
}
actions["force-promote"] = {
  ["disabled"] = true,
  ["displayName"] = "Force Promote",
  ["iconClass"] = "fa-solid fa-fw fa-forward"
}
actions["force-promote-chain"] = {
  ["disabled"] = true,
  ["displayName"] = "Force Promote Chain",
  ["iconClass"] = "fa-solid fa-fw fa-forward-fast"
}

-- identifies if a MonoVertex is owned by a parent MonoVertexRollout
-- if it is, we disable the pause/unpause actions on the MonoVertex
-- to avoid Numaplane self-healing over changes
function isChild(obj)
  if obj.metadata ~= nil and obj.metadata.ownerReferences ~= nil then
    for i, ownerRef in ipairs(obj.metadata.ownerReferences) do
      if ownerRef["kind"] == "MonoVertexRollout" then
        return true
      end
    end
  end
  return false
end

-- pause/unpause
if isChild(obj) == false then
  local paused = false
  if obj.spec.lifecycle ~= nil and obj.spec.lifecycle.desiredPhase ~= nil and obj.spec.lifecycle.desiredPhase == "Paused" then
    paused = true
  end
  if paused then
    if obj.spec.metadata ~= nil and  obj.spec.metadata.annotations ~= nil and obj.spec.metadata.annotations["numaflow.numaproj.io/allowed-resume-strategies"] ~= nil then
      -- determine which unpausing strategies will be enabled
      -- if annotation not found, default will be resume slow
      if obj.spec.metadata.annotations["numaflow.numaproj.io/allowed-resume-strategies"] == "fast" then
        actions["unpause-fast"]["disabled"] = false
      elseif obj.spec.metadata.annotations["numaflow.numaproj.io/allowed-resume-strategies"] == "slow, fast" then
        actions["unpause-gradual"]["disabled"] = false
        actions["unpause-fast"]["disabled"] = false
      else
        actions["unpause-gradual"]["disabled"] = false
      end
    else
      actions["unpause-gradual"]["disabled"] = false
    end
  else
    actions["pause"]["disabled"] = false
  end
end

-- force-promote / force-promote-chain
-- Numaplane ignores an individual force-promote while the trial chain has failed,
-- so only one of the two actions is enabled at a time
local labels = obj.metadata.labels or {}
local isTrial = labels["numaplane.numaproj.io/upgrade-state"] == "in-progress" or labels["numaplane.numaproj.io/upgrade-state"] == "trial"
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