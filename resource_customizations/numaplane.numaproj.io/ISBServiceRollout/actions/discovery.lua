local actions = {}
actions["add-full-promote"] = {["disabled"] = true}
actions["remove-full-promote"] = {["disabled"] = true}

-- full-promote
if (obj.status ~= nil and obj.status.upgradeInProgress == "Progressive" and obj.status.phase == "Pending") then
    actions["add-full-promote"]["disabled"] = false
end
if obj.metadata.labels ~= nil and obj.metadata.labels["numaplane.numaproj.io/promote"] == "true" then
  actions["remove-full-promote"]["disabled"] = false
end
  
return actions