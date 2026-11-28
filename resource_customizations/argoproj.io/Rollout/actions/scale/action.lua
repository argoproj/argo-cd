local replicasParam = actionParams["replicas"]
if replicasParam == nil then
    error("replicas parameter is required", 0)
end

local replicas = tonumber(replicasParam)
if not replicas or replicas < 0 or replicas % 1 ~= 0 or replicas > 2147483647 then
    error("invalid number: " .. tostring(replicasParam), 0)
end

if obj.spec == nil then
    obj.spec = {}
end
obj.spec.replicas = replicas
return obj
