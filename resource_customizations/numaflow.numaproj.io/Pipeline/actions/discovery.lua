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

-- pause/unpause
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

-- force-promote
local forcePromote = false
if (obj.metadata.labels ~= nil and obj.metadata.labels["numaplane.numaproj.io/upgrade-state"] == "in-progress") then
  forcePromote = true
end
if (obj.metadata.labels ~= nil and obj.metadata.labels["numaplane.numaproj.io/force-promote"] == "true") then
  forcePromote = false
end
if forcePromote then
  actions["force-promote"]["disabled"] = false
else
  actions["force-promote"]["disabled"] = true