local actions = {}
actions["pause"] = {["disabled"] = true}
actions["unpause"] = {["disabled"] = true}
actions["allow-data-loss"] = {["disabled"] = true}
actions["disallow-data-loss"] = {["disabled"] = true}
actions["enable-force-promote"] = {
  ["disabled"] = true,
  ["displayName"] = "Enable Force Promote"
}
actions["disable-force-promote"] = {
  ["disabled"] = true,
  ["displayName"] = "Disable Force Promote"
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

-- force-promote
if (obj.status ~= nil and obj.status.upgradeInProgress == "Progressive" and obj.status.phase == "Pending") then
  actions["enable-force-promote"]["disabled"] = false
end
if (obj.spec ~= nil and obj.spec.strategy ~= nil and obj.spec.strategy.progressive ~= nil and obj.spec.strategy.progressive.forcePromote == true) then
  actions["disable-force-promote"]["disabled"] = false
end

return actions