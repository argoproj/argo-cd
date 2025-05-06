local os = require("os")

local replicas = tonumber(actionParams["scale"])
if not replicas then
    error("invalid number: " .. actionParams["scale"], 0)
end

obj.spec.replicas = replicas
return obj
