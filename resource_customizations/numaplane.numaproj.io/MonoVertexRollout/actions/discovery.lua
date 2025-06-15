local actions = {}
actions["pause"] = {
  ["disabled"] = true,
  ["iconClass"] = "fa-solid fa-fw fa-pause"
}
actions["unpause"] = {
  ["disabled"] = true,
  ["iconClass"] = "fa-solid fa-fw fa-play"
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

return actions