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

-- pause/unpause
local paused = false
if obj.spec.monoVertex.spec.lifecycle ~= nil and obj.spec.monoVertex.spec.lifecycle.desiredPhase ~= nil and obj.spec.monoVertex.spec.lifecycle.desiredPhase == "Paused" then
  paused = true
end
if paused then
  if obj.spec.monoVertex.spec.metadata ~= nil and  obj.spec.monoVertex.spec.metadata.annotations ~= nil and obj.spec.monoVertex.spec.metadata.annotations["numaflow.numaproj.io/allowed-resume-strategies"] ~= nil then
    -- determine which unpausing strategies will be enabled
    -- if annotation not found, default will be resume slow
    if obj.spec.monoVertex.spec.metadata.annotations["numaflow.numaproj.io/allowed-resume-strategies"] == "fast" then
      actions["unpause-fast"]["disabled"] = false
    elseif obj.spec.monoVertex.spec.metadata.annotations["numaflow.numaproj.io/allowed-resume-strategies"] == "slow, fast" then
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

return actions