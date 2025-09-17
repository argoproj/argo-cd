local os = require("os")

local replicas = tonumber(actionParams["replicas"])
if not replicas then
    error("invalid number: " .. actionParams["replicas"], 0)
end

obj.spec.replicas = replicas
return obj
