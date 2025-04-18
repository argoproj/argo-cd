local os = require("os")

obj.spec.replicas = tonumber(actionParams["scale"])
return obj
