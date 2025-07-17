local actions = {}
actions["pause"] = {["disabled"] = true}
actions["unpause"] = {["disabled"] = true}
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
if obj.spec.monoVertex.spec.lifecycle ~= nil and obj.spec.monoVertex.spec.lifecycle.desiredPhase ~= nil and obj.spec.monoVertex.spec.lifecycle.desiredPhase == "Paused" then
  paused = true
end
if paused then
  actions["unpause"]["disabled"] = false
else
  actions["pause"]["disabled"] = false
end

-- force-promote
if (obj.status ~= nil and obj.status.upgradeInProgress == "Progressive" and obj.status.phase == "Pending") then
  actions["enable-force-promote"]["disabled"] = false
end
if (obj.spec ~= nil and obj.spec.strategy ~= nil and obj.spec.strategy.progressive ~= nil and obj.spec.strategy.progressive.forcePromote == true) then
  actions["disable-force-promote"]["disabled"] = false
end

return actions