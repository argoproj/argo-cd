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
actions["allow-data-loss"] = {
  ["disabled"] = true,
  ["displayName"] = "Allow (Possible) Data Loss",
  ["iconClass"] = "fa-solid fa-fw fa-unlock"
}
actions["disallow-data-loss"] = {
  ["disabled"] = true,
  ["displayName"] = "Disallow Data Loss",
  ["iconClass"] = "fa-solid fa-fw fa-lock"
}

-- pause/unpause
local paused = false
if obj.spec.pipeline.spec.lifecycle ~= nil and obj.spec.pipeline.spec.lifecycle.desiredPhase ~= nil and obj.spec.pipeline.spec.lifecycle.desiredPhase == "Paused" then
  paused = true
end
if paused then
  if obj.spec.pipeline.metadata ~= nil and  obj.spec.pipeline.metadata.annotations ~= nil and obj.spec.pipeline.metadata.annotations["numaflow.numaproj.io/allowed-resume-strategies"] ~= nil then
    -- determine which unpausing strategies will be enabled
    -- if annotation not found, default will be resume slow
    if obj.spec.pipeline.metadata.annotations["numaflow.numaproj.io/allowed-resume-strategies"] == "fast" then
      actions["unpause-fast"]["disabled"] = false
    elseif obj.spec.pipeline.metadata.annotations["numaflow.numaproj.io/allowed-resume-strategies"] == "slow, fast" then
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

-- allow-data-loss/disallow-data-loss
if obj.status ~= nil and obj.status.upgradeInProgress == "PipelinePauseAndDrain" then
  actions["allow-data-loss"]["disabled"] = false
end
if obj.metadata.annotations ~= nil and obj.metadata.annotations["numaplane.numaproj.io/allow-data-loss"] == "true" then
  actions["disallow-data-loss"]["disabled"] = false
end

return actions