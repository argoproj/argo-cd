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
  actions["unpause-gradual"]["disabled"] = false
  actions["unpause-fast"]["disabled"] = false
else
  actions["pause"]["disabled"] = false
end

return actions