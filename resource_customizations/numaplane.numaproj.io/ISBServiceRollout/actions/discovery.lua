local actions = {}
actions["enable-force-promote"] = {
  ["disabled"] = true,
  ["displayName"] = "Enable Force Promote"
}
actions["disable-force-promote"] = {
  ["disabled"] = true,
  ["displayName"] = "Disable Force Promote"
}

-- force-promote
if (obj.status ~= nil and obj.status.upgradeInProgress == "Progressive" and obj.status.phase == "Pending") then
  actions["enable-force-promote"]["disabled"] = false
end
if (obj.spec ~= nil and obj.spec.strategy ~= nil and obj.spec.strategy.progressive ~= nil and obj.spec.strategy.progressive.forcePromote == true) then
  actions["disable-force-promote"]["disabled"] = false
end
  
return actions