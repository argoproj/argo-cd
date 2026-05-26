local os = require("os")

local replicas = actionParams["replicas"]
if not replicas then
    error("replicas parameter is required", 0)
end

local replicasNum = tonumber(replicas)
if not replicasNum or replicasNum < 0 then
    error("invalid number: " .. replicas .. " (must be >= 0)", 0)
end

obj.spec.replicas = replicasNum
return obj
