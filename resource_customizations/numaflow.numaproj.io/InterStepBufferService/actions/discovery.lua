local actions = {}
actions["force-promote"] = {
  ["disabled"] = true,
  ["iconClass"] = "fa-solid fa-fw fa-forward"
}

-- force-promote
local forcePromote = false
if obj.metadata.labels ~= nil and (obj.metadata.labels["numaplane.numaproj.io/upgrade-state"] == "in-progress" or obj.metadata.labels["numaplane.numaproj.io/upgrade-state"] == "trial") then
  forcePromote = true
end
if (obj.metadata.labels ~= nil and obj.metadata.labels["numaplane.numaproj.io/force-promote"] == "true") then
  forcePromote = false
end
if forcePromote then
  actions["force-promote"]["disabled"] = false
else
  actions["force-promote"]["disabled"] = true
end

return actions