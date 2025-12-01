local obj = obj or {}
if obj.metadata == nil then obj.metadata = {} end
if obj.metadata.annotations == nil then obj.metadata.annotations = {} end

local replicas = actionParams["replicas"]
if not replicas then
    error("replicas parameter is required", 0)
end

local replicasNum = tonumber(replicas)
if not replicasNum or replicasNum < 0 then
    error("invalid number: " .. replicas .. " (must be >= 0)", 0)
end

obj.metadata.annotations["autoscaling.keda.sh/paused-replicas"] = tostring(replicasNum)

return obj
