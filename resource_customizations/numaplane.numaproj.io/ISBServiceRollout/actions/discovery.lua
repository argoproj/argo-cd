local actions = {}
actions["enable-full-promote"] = {["disabled"] = true}
actions["disable-full-promote"] = {["disabled"] = true}

-- full-promote
if (obj.status ~= nil and obj.status.upgradeInProgress == "Progressive" and obj.status.phase == "Pending") then
    actions["enable-full-promote"]["disabled"] = false
end
if obj.metadata.labels ~= nil and obj.metadata.labels["numaplane.numaproj.io/promote"] == "true" then
  actions["disable-full-promote"]["disabled"] = false
end
  
return actions