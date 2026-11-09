local health_status = {
    status = "Progressing",
    message = "Waiting for Sealed Secret to be decrypted"
}

-- A Synced condition can still describe an earlier generation.
if obj.metadata == nil or obj.metadata.generation == nil or obj.status == nil or
    obj.status.observedGeneration ~= obj.metadata.generation then
    return health_status
end

if obj.status.conditions ~= nil then
    for _, condition in ipairs(obj.status.conditions) do
        if condition.type == "Synced" and condition.status == "False" then
            health_status.status = "Degraded"
            health_status.message = condition.message
            return health_status
        end
        if condition.type == "Synced" and condition.status == "True" then
            health_status.status = "Healthy"
            health_status.message = condition.message
            return health_status
        end
    end
end
return health_status
