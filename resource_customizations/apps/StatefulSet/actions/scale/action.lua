local os = require("os")

local replicas = actionParams["replicas"]
if not replicas then
    error("replicas parameter is required", 0)
end

local replicasNum = tonumber(replicas)
if not replicasNum or replicasNum < 0 or replicasNum %1 ~=0 then
    error("invalid number: " .. replicas .. " (must be a non-negative integer)", 0)
end

obj.spec.replicas = replicasNum
return obj
