local health_status={}
if obj.status ~= nil then
    if obj.status.conditions ~= nil then
        for i, condition in ipairs(obj.status.conditions) do
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
end
health_status.status = "Progressing"
health_status.message = "Waiting for Sealed Secret to be decrypted"
return health_status
