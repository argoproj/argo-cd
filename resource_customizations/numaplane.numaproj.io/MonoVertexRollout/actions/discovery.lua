local actions = {}
actions["pause"] = {["disabled"] = true}
actions["unpause"] = {["disabled"] = true}
actions["add-full-promote"] = {["disabled"] = true}
actions["remove-full-promote"] = {["disabled"] = true}

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

-- full-promote
if (obj.status ~= nil and obj.status.upgradeInProgress == "Progressive" and obj.status.phase == "Pending") then
  actions["add-full-promote"]["disabled"] = false
end
if obj.metadata.labels ~= nil and obj.metadata.labels["numaplane.numaproj.io/promote"] == "true" then
  actions["remove-full-promote"]["disabled"] = false
end

return actions