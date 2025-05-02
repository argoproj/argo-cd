local actions = {}
actions["pause"] = {
  ["disabled"] = true,
  ["iconClass"] = "fa-solid fa-fw fa-pause"
}
actions["unpause"] = {
  ["disabled"] = true,
  ["iconClass"] = "fa-solid fa-fw fa-play"
}
actions["force-promote"] = {
  ["disabled"] = true,
  ["iconClass"] = "fa-solid fa-fw fa-forward"
}

-- pause/unpause
local paused = false
if obj.spec.lifecycle ~= nil and obj.spec.lifecycle.desiredPhase ~= nil and obj.spec.lifecycle.desiredPhase == "Paused" then
  paused = true
end
if paused then
  actions["unpause"]["disabled"] = false
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