local actions = {}
actions["pause"] = {
  ["disabled"] = true,
  ["iconClass"] = "fa-solid fa-fw fa-pause"
}
actions["unpause"] = {
  ["disabled"] = true,
  ["iconClass"] = "fa-solid fa-fw fa-play"
}
actions["allow-data-loss"] = {
  ["disabled"] = true,
  ["displayName"] = "Allow Data Loss",
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
  actions["unpause"]["disabled"] = false
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